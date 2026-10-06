package build

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// `Option<T>` is the builtin enum `None | Some(T)`.
//
// Its type string is `Option<` + the type string of T + `>`. Its Go type has
// the layout of every compiled enum, a tag and a payload, and is declared
// once for each option type a program names: generated Go is monomorphic,
// with no Go generics and no runtime type dictionary. The Go name spells the
// payload's type string reversibly, so the emitter finds the option types a
// program uses in the Go it has just written and declares exactly those
// (appendOptionDeclarations); nothing has to be collected while lowering.
// T is known at every construction because the typechecker writes it there
// (ast.AsOptionConstruction).

const (
	optionTypeStringPrefix = ast.OptionTypeName + "<"
	goOptionTypePrefix     = "__octOption_"
)

// parseOptionType is T for the type string `Option<T>`. `Option<T>[]` and
// `fn() -> Option<T>` are not option types.
func parseOptionType(t string) (string, bool) {
	if !strings.HasPrefix(t, optionTypeStringPrefix) || !strings.HasSuffix(t, ">") {
		return "", false
	}
	// The `>` that ends the string must be the one that closes `Option<`.
	depth := 0
	for index := 0; index < len(t); index++ {
		switch t[index] {
		case '<':
			depth++
		case '>':
			if index > 0 && t[index-1] == '-' {
				continue // the arrow of a function type
			}
			depth--
			if depth == 0 && index != len(t)-1 {
				return "", false
			}
		}
	}
	if depth != 0 {
		return "", false
	}
	return t[len(optionTypeStringPrefix) : len(t)-1], true
}

func optionTypeString(payloadType string) string {
	return optionTypeStringPrefix + payloadType + ">"
}

// goOptionType is the Go type of `Option<payloadType>`: the prefix, then the
// payload's type string with every character that is not a letter or a digit
// written as `_` and two hexadecimal digits.
func goOptionType(payloadType string) string {
	var name strings.Builder
	name.WriteString(goOptionTypePrefix)
	for index := 0; index < len(payloadType); index++ {
		ch := payloadType[index]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			name.WriteByte(ch)
			continue
		}
		fmt.Fprintf(&name, "_%02x", ch)
	}
	return name.String()
}

// optionPayloadOfGoType reads the payload's type string back out of a Go
// name that goOptionType made.
func optionPayloadOfGoType(goName string) (string, bool) {
	if !strings.HasPrefix(goName, goOptionTypePrefix) {
		return "", false
	}
	encoded := goName[len(goOptionTypePrefix):]
	var payload strings.Builder
	for index := 0; index < len(encoded); index++ {
		if encoded[index] != '_' {
			payload.WriteByte(encoded[index])
			continue
		}
		if index+2 >= len(encoded) {
			return "", false
		}
		value, err := strconv.ParseUint(encoded[index+1:index+3], 16, 8)
		if err != nil {
			return "", false
		}
		payload.WriteByte(byte(value))
		index += 2
	}
	if payload.Len() == 0 {
		return "", false
	}
	return payload.String(), true
}

var goOptionTypeName = regexp.MustCompile(`\b` + goOptionTypePrefix + `[A-Za-z0-9_]+`)

// appendOptionDeclarations declares the option types that generated Go
// names, and with the first of them the tags and the equality that every
// option shares. A program that names no option is returned as it is.
func appendOptionDeclarations(src string) string {
	declared := map[string]struct{}{}
	var declarations strings.Builder
	withOctagon := strings.Contains(src, "func __octEnumMetaOf(")
	// Declaring one option can name another: the payload of an
	// Option<Option<Int>> is an option type the program may never spell.
	for scanned := src; ; scanned = declarations.String() {
		names := goOptionTypeName.FindAllString(scanned, -1)
		sort.Strings(names)
		added := false
		for _, name := range names {
			if _, done := declared[name]; done {
				continue
			}
			declared[name] = struct{}{}
			payloadType, ok := optionPayloadOfGoType(name)
			if !ok {
				continue
			}
			added = true
			fmt.Fprintf(&declarations, "type %s struct {\n\tTag     int\n\tPayload any\n}\n\n", name)
			if withOctagon {
				// The Octagon loader asks an option for the Go type of its
				// payload; see optionOctagonRuntime.
				fmt.Fprintf(&declarations, "func (%s) __octOptionPayloadType() reflect.Type {\n\treturn reflect.TypeOf((*%s)(nil)).Elem()\n}\n\n", name, goType(payloadType))
			}
		}
		if !added {
			break
		}
	}
	if declarations.Len() == 0 {
		return src
	}
	return src + "\n" + optionRuntime + declarations.String()
}

// optionRuntime is emitted once into a program that uses an option: the tags
// of the two variants, and the equality that `==` and `!=` on two options
// compile to. The equality is by value at any depth, as the interpreter's is;
// Go's `==` would panic on a payload that holds an array.
const optionRuntime = `const (
	Option_None_tag = 0
	Option_Some_tag = 1
)

func __octOptionEqual(a, b any) bool { return __octValueEqual(reflect.ValueOf(a), reflect.ValueOf(b)) }

func __octValueEqual(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Bool:
		return a.Bool() == b.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.Uint() == b.Uint()
	case reflect.Float32, reflect.Float64:
		return a.Float() == b.Float()
	case reflect.Complex64, reflect.Complex128:
		return a.Complex() == b.Complex()
	case reflect.String:
		return a.String() == b.String()
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !__octValueEqual(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !__octValueEqual(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return __octValueEqual(a.Elem(), b.Elem())
	default:
		return a.CanInterface() && b.CanInterface() && reflect.DeepEqual(a.Interface(), b.Interface())
	}
}

`

// optionOctagonRuntime is emitted with the Octagon loader and writer. An
// option type has no entry in the table of declared enums, so its
// description is made from the type itself: the Go type gives the payload's
// Go type, and the Oct type string gives the payload's Oct type, which is
// what carries a dimension.
const optionOctagonRuntime = `type __octOptionLike interface{ __octOptionPayloadType() reflect.Type }

func __octEnumMetaOf(t reflect.Type, expectedType string) (__octEnumMeta, bool) {
	if meta, ok := __octEnumMetaByGoType[__octTypeKey(t)]; ok {
		return meta, true
	}
	if t.Kind() != reflect.Struct {
		return __octEnumMeta{}, false
	}
	option, ok := reflect.New(t).Elem().Interface().(__octOptionLike)
	if !ok {
		return __octEnumMeta{}, false
	}
	payloadName := ""
	if strings.HasPrefix(expectedType, "Option<") && strings.HasSuffix(expectedType, ">") {
		payloadName = expectedType[len("Option<") : len(expectedType)-1]
	}
	return __octEnumMeta{FullName: "Option", ShortName: "Option", Variants: []string{"None", "Some"}, PayloadTypes: []reflect.Type{nil, option.__octOptionPayloadType()}, PayloadNames: []string{"", payloadName}}, true
}

`

// lowerOptionConstruction lowers `Option<T>.None` and `Option<T>.Some(x)`.
func (c *lowerCtx) lowerOptionConstruction(construction ast.OptionConstruction) (string, string, bool, error) {
	if !construction.Resolved {
		return "", "", false, fmt.Errorf("internal error: `Option.%s` reached lowering without the type the typechecker gives it", construction.Variant)
	}
	payloadType := typeRefStringForPackage(c.pkg.Name, construction.Payload)
	optionType := optionTypeString(payloadType)
	switch construction.Variant {
	case ast.OptionNoneVariant:
		// The parser gives `None` no argument list.
		return fmt.Sprintf("%s{Tag: Option_None_tag}", goOptionType(payloadType)), optionType, false, nil
	case ast.OptionSomeVariant:
		if len(construction.Arguments) != 1 {
			return "", "", false, fmt.Errorf("enum 'Option' variant 'Some' requires exactly 1 payload argument")
		}
		// T decides what the payload is, as a parameter's type does for an
		// argument, and the option holds its own copy of an array.
		payload, actualType, _, err := c.withExpectedType(payloadType, func() (string, string, bool, error) {
			return c.lowerExpr(construction.Arguments[0])
		})
		if err != nil {
			return "", "", false, err
		}
		payload = cloneCompiledValueExpr(coerceExprToType(payload, actualType, c.eraseRefinementType(payloadType)), payloadType)
		return fmt.Sprintf("%s{Tag: Option_Some_tag, Payload: %s}", goOptionType(payloadType), payload), optionType, false, nil
	default:
		return "", "", false, fmt.Errorf("enum 'Option' has no variant '%s'", construction.Variant)
	}
}

// optionCaseLabel reports whether a switch case label is `Option.<variant>`.
func optionCaseLabel(label ast.Expr) (string, bool) {
	access, ok := label.(ast.FieldAccessExpr)
	if !ok {
		return "", false
	}
	target, ok := access.Target.(ast.IdentifierExpr)
	if !ok || target.Name != ast.OptionTypeName {
		return "", false
	}
	return access.Field, true
}
