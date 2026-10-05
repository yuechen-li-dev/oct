package interpret

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// A declaration decides what a value is. `let x: Float = 1` declares a Float
// equal to one, and a parameter, a result, a field, a column, an enum payload,
// a yield and a turn input declare their types in the same way. The
// typechecker accepts an Int where a Float is declared, and an Int or a Float
// where a Complex is (isAssignable in internal/typecheck). The value that
// arrives is then the declared kind, here as in the compiled lane; without
// this an Int would stay an Int, and `x / 2` would divide integers.
//
// The interpreter has no static types, so it converts where the declared type
// is at hand (conformToDeclared), and where it is not, by the value being
// replaced (conformLike).

// conform answers with value as declared has it, in the package the
// declaration was written in. It is conformToDeclared with one addition: a
// refined concept is the type it refines.
func (i interpreter) conform(value Value, declared ast.TypeRef, pkgName string) Value {
	if value.Kind == ValueInt || value.Kind == ValueFloat {
		declared = i.representation(declared, pkgName)
	}
	return conformToDeclared(value, declared)
}

// representation is the type a refined scalar concept refines. Any other type
// is returned as it is. Aliases need nothing here: they are expanded before
// the program runs.
func (i interpreter) representation(declared ast.TypeRef, pkgName string) ast.TypeRef {
	if len(i.refinements) == 0 || declared.Name == "" || declared.IsArray || declared.ArrayDepth > 0 {
		return declared
	}
	switch declared.Name {
	case "Int", "Float", "Complex", "Bool", "String":
		return declared
	}
	if declared.Package != "" {
		pkgName = declared.Package
	}
	if concept, found := i.refinements[pkgName+"."+declared.Name]; found {
		return concept.Target
	}
	return declared
}

// conformToDeclared answers with value as the declared type has it. A value
// that already is what the declaration says is returned unchanged, without
// being copied.
func conformToDeclared(value Value, declared ast.TypeRef) Value {
	if !declaresWidening(declared) {
		return value
	}
	if declared.IsArray || declared.ArrayDepth > 0 {
		if value.Kind != ValueArray {
			return value
		}
		// The elements of an array share one type, so the first of them says
		// whether any needs converting. Most arrays that cross a declaration
		// are already what it says, and this keeps that case from costing a
		// pass over the array.
		leaf, found := firstLeaf(value)
		if !found {
			return value
		}
		switch leaf.Kind {
		case ValueInt, ValueFloat:
			if !widens(leaf, declared.Name) {
				return value
			}
		case ValueVector, ValueMatrix, ValueTuple:
		default:
			return value
		}
		element := declared
		element.ArrayDepth--
		element.IsArray = element.ArrayDepth > 0
		return Value{Kind: ValueArray, Array: conformElements(value.Array, func(item Value) Value {
			return conformToDeclared(item, element)
		})}
	}
	switch {
	case len(declared.TupleOf) > 0:
		if value.Kind != ValueTuple || len(value.Tuple) != len(declared.TupleOf) {
			return value
		}
		index := -1
		value.Tuple = conformElements(value.Tuple, func(item Value) Value {
			index++
			return conformToDeclared(item, declared.TupleOf[index])
		})
		return value
	case declared.VectorOf != nil:
		if value.Kind != ValueVector || len(value.Vector) == 0 || !widens(value.Vector[0], declared.VectorOf.Name) {
			return value
		}
		value.Vector = conformElements(value.Vector, func(item Value) Value {
			return conformToDeclared(item, *declared.VectorOf)
		})
		return value
	case declared.MatrixOf != nil:
		if value.Kind != ValueMatrix || len(value.Matrix.Elements) == 0 || !widens(value.Matrix.Elements[0], declared.MatrixOf.Name) {
			return value
		}
		value.Matrix.Elements = conformElements(value.Matrix.Elements, func(item Value) Value {
			return conformToDeclared(item, *declared.MatrixOf)
		})
		return value
	}
	return widenScalar(value, declared.Name)
}

// conformLike answers with value as the value it replaces has it. It serves
// the assignments whose target has no written type at hand: a variable whose
// type was inferred, and an element or a row of an array.
func conformLike(value Value, current Value) Value {
	switch current.Kind {
	case ValueFloat, ValueComplex:
		return conformLeaves(value, current)
	case ValueArray:
		// The elements of an array share one type, so any element of the
		// array being replaced says what the new elements are.
		if model, found := firstLeaf(current); found {
			return conformLeaves(value, model)
		}
	case ValueVector:
		if value.Kind == ValueVector && len(current.Vector) > 0 {
			value.Vector = conformElements(value.Vector, func(item Value) Value {
				return conformLeaves(item, current.Vector[0])
			})
		}
	case ValueMatrix:
		if value.Kind == ValueMatrix && len(current.Matrix.Elements) > 0 {
			value.Matrix.Elements = conformElements(value.Matrix.Elements, func(item Value) Value {
				return conformLeaves(item, current.Matrix.Elements[0])
			})
		}
	}
	return value
}

// conformLeaves answers with the scalars of value, at any depth of array, as
// the kind of model: Int becomes Float where model is a Float, and Int and
// Float become Complex where model is a Complex.
func conformLeaves(value Value, model Value) Value {
	name := ""
	switch model.Kind {
	case ValueFloat:
		name = "Float"
	case ValueComplex:
		name = "Complex"
	default:
		return value
	}
	if value.Kind != ValueArray {
		return widenScalar(value, name)
	}
	if leaf, found := firstLeaf(value); !found || !widens(leaf, name) {
		return value
	}
	return Value{Kind: ValueArray, Array: conformElements(value.Array, func(item Value) Value {
		return conformLeaves(item, model)
	})}
}

// unifyNumericElements serves an array literal whose elements are not all one
// kind. The typechecker admits that only where the literal is declared to
// hold Float or Complex, as in `let xs: Float[] = [1, 2.5]`, so the elements
// become the widest kind among them. The second result is false when the
// elements differ in some other way.
func unifyNumericElements(elements []Value) ([]Value, bool) {
	widest := Value{Kind: ValueInt}
	for _, element := range elements {
		leaf, found := firstLeaf(element)
		if !found {
			continue
		}
		switch leaf.Kind {
		case ValueComplex:
			widest = leaf
		case ValueFloat:
			if widest.Kind != ValueComplex {
				widest = leaf
			}
		case ValueInt:
		default:
			return elements, false
		}
	}
	if widest.Kind == ValueInt {
		return elements, false
	}
	return conformElements(elements, func(item Value) Value {
		return conformLeaves(item, widest)
	}), true
}

// declaresWidening reports whether a declared type has a Float or a Complex
// in it, which is the only case in which a value can need converting.
func declaresWidening(declared ast.TypeRef) bool {
	switch {
	case declared.Function != nil, declared.FlowInstanceOf != nil:
		return false
	case declared.VectorOf != nil:
		return declaresWidening(*declared.VectorOf)
	case declared.MatrixOf != nil:
		return declaresWidening(*declared.MatrixOf)
	case len(declared.TupleOf) > 0:
		for _, element := range declared.TupleOf {
			if declaresWidening(element) {
				return true
			}
		}
		return false
	}
	return declared.Package == "" && (declared.Name == "Float" || declared.Name == "Complex")
}

// widens reports whether a scalar is converted by a declaration of the named
// type.
func widens(value Value, declared string) bool {
	switch declared {
	case "Float":
		return value.Kind == ValueInt
	case "Complex":
		return value.Kind == ValueInt || value.Kind == ValueFloat
	}
	return false
}

func widenScalar(value Value, declared string) Value {
	switch declared {
	case "Float":
		if value.Kind == ValueInt {
			return Value{Kind: ValueFloat, Float: float64(value.Int), Dimension: value.Dimension}
		}
	case "Complex":
		switch value.Kind {
		case ValueInt:
			return Value{Kind: ValueComplex, Complex: complex(float64(value.Int), 0)}
		case ValueFloat:
			return Value{Kind: ValueComplex, Complex: complex(value.Float, 0)}
		}
	}
	return value
}

// conformElements applies convert to each element. The slice it was given is
// returned as it is when no element changes, which is the usual case and
// costs one comparison for each element converted and none copied.
func conformElements(elements []Value, convert func(Value) Value) []Value {
	var converted []Value
	for index, element := range elements {
		next := convert(element)
		if converted == nil {
			if sameScalarKind(next, element) {
				continue
			}
			converted = make([]Value, len(elements))
			copy(converted, elements[:index])
		}
		converted[index] = next
	}
	if converted == nil {
		return elements
	}
	return converted
}

// sameScalarKind reports whether conversion left an element as it was. An
// element that is itself a collection is compared by its storage: a
// collection that needed no conversion comes back as the same slice.
func sameScalarKind(next Value, element Value) bool {
	if next.Kind != element.Kind {
		return false
	}
	switch next.Kind {
	case ValueArray:
		return sameStorage(next.Array, element.Array)
	case ValueVector:
		return sameStorage(next.Vector, element.Vector)
	case ValueMatrix:
		return sameStorage(next.Matrix.Elements, element.Matrix.Elements)
	case ValueTuple:
		return sameStorage(next.Tuple, element.Tuple)
	}
	return true
}

func sameStorage(a []Value, b []Value) bool {
	if len(a) != len(b) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// firstLeaf is the first value of an array that is not itself an array.
func firstLeaf(value Value) (Value, bool) {
	if value.Kind != ValueArray {
		return value, true
	}
	for _, element := range value.Array {
		if leaf, found := firstLeaf(element); found {
			return leaf, true
		}
	}
	return Value{}, false
}

// conformAssigned answers with value as the variable it is assigned to has
// it: by the type the variable was written with, or, when it was written with
// none, by the value it holds.
func (i interpreter) conformAssigned(env *environment, pkgName string, name string, value Value) Value {
	target, found := env.lookup(name)
	if !found {
		return value
	}
	if target.declared != nil {
		return i.conform(value, *target.declared, pkgName)
	}
	return conformLike(value, target.value)
}

// conformIndexed answers with value as an element of container has it, for
// an assignment through indices. The container says what its elements are:
// an index names an element that exists, so an array that is assigned into
// has a value in it, and the elements of an array share one type.
func conformIndexed(value Value, container Value) Value {
	if container.Kind == ValueMatrix {
		if len(container.Matrix.Elements) == 0 {
			return value
		}
		return conformLeaves(value, container.Matrix.Elements[0])
	}
	return conformLike(value, container)
}

// conformField answers with value as the named field of a record has it, for
// a `with` update. current is the value the field holds.
func (i interpreter) conformField(value Value, current Value, record ast.RecordDecl, known bool, name string, pkgName string) Value {
	if known {
		for _, field := range record.Fields {
			if field.Name == name {
				return i.conform(value, storedFieldType(record, field), pkgName)
			}
		}
	}
	return conformLike(value, current)
}

// storedFieldType is the type of what a record holds for a field: the field's
// type, or for a record table a column of it.
func storedFieldType(record ast.RecordDecl, field ast.RecordField) ast.TypeRef {
	stored := field.Type
	if record.IsTable {
		stored.ArrayDepth++
		stored.IsArray = true
	}
	return stored
}

// boardFieldType is the declared type of a field of the board of the flow
// that env belongs to, or nil when target is not that board.
func boardFieldType(env *environment, target string, field string) *ast.TypeRef {
	if target != "board" {
		return nil
	}
	instance, err := flowInstanceFromEnv(env)
	if err != nil {
		return nil
	}
	for index := range instance.Decl.Board {
		if instance.Decl.Board[index].Name == field {
			return &instance.Decl.Board[index].Type
		}
	}
	return nil
}

// sameKindElements answers with the elements of an array literal once they
// are all one kind, converting an Int beside a Float as unifyNumericElements
// describes. Elements that still differ are a typechecker failure.
func sameKindElements(elements []Value) ([]Value, error) {
	if kind, other, mixed := mixedKinds(elements); mixed {
		unified, numeric := unifyNumericElements(elements)
		if !numeric {
			return nil, fmt.Errorf("runtime invariant violation: array literal has mixed element kinds %s and %s", kind, other)
		}
		if kind, other, mixed = mixedKinds(unified); mixed {
			return nil, fmt.Errorf("runtime invariant violation: array literal has mixed element kinds %s and %s", kind, other)
		}
		return unified, nil
	}
	return elements, nil
}

func mixedKinds(elements []Value) (string, string, bool) {
	var first string
	for index, element := range elements {
		name := valueTypeName(element)
		if index == 0 {
			first = name
		} else if name != first {
			return first, name, true
		}
	}
	return "", "", false
}
