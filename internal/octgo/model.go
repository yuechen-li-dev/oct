// Package octgo implements the deliberately bounded Go-companion experiment.
// It projects selected, typed Go declarations into a deterministic semantic IR;
// it is not a Go AST or runtime-reflection surface.
package octgo

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

type PackageIdentity struct {
	Path string
	Name string
}

type Position struct {
	File   string
	Line   int
	Column int
}

type TypeRef struct {
	Kind    string
	Name    string
	Package string
}

func (t TypeRef) String() string {
	if t.Name != "" {
		if t.Package != "" {
			return t.Package + "." + t.Name
		}
		return t.Name
	}
	return t.Kind
}

type FieldModel struct {
	Name string
	Type TypeRef
}

type TypeModel struct {
	Name              string
	Kind              string
	Underlying        TypeRef
	Fields            []FieldModel
	Alias             bool
	Position          Position
	Supported         bool
	UnsupportedReason string
}

type ConstantModel struct {
	Name              string
	Type              TypeRef
	Value             string
	OctLiteral        string
	Position          Position
	Supported         bool
	UnsupportedReason string
}

type FunctionModel struct {
	Name              string
	Parameters        []TypeRef
	Results           []TypeRef
	Variadic          bool
	Position          Position
	Signature         string
	Supported         bool
	UnsupportedReason string
}

type PackageModel struct {
	Package   PackageIdentity
	Directory string
	Types     []TypeModel
	Constants []ConstantModel
	Functions []FunctionModel
}

func LoadPackage(directory string) (PackageModel, error) {
	cfg := &packages.Config{
		Dir:  directory,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
	}
	loaded, err := packages.Load(cfg, ".")
	if err != nil {
		return PackageModel{}, fmt.Errorf("load Go package %s: %w", directory, err)
	}
	if packages.PrintErrors(loaded) > 0 {
		return PackageModel{}, fmt.Errorf("load Go package %s: package has errors", directory)
	}
	if len(loaded) != 1 || loaded[0].Types == nil {
		return PackageModel{}, fmt.Errorf("load Go package %s: expected exactly one typed package, got %d", directory, len(loaded))
	}
	pkg := loaded[0]
	model := PackageModel{
		Package:   PackageIdentity{Path: pkg.PkgPath, Name: pkg.Name},
		Directory: directory,
	}
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		switch decl := obj.(type) {
		case *types.TypeName:
			model.Types = append(model.Types, typeModel(pkg.Fset, decl))
		case *types.Const:
			model.Constants = append(model.Constants, constantModel(pkg.Fset, decl))
		case *types.Func:
			model.Functions = append(model.Functions, functionModel(pkg.Fset, decl))
		}
	}
	sort.Slice(model.Types, func(i, j int) bool { return model.Types[i].Name < model.Types[j].Name })
	sort.Slice(model.Constants, func(i, j int) bool { return model.Constants[i].Name < model.Constants[j].Name })
	sort.Slice(model.Functions, func(i, j int) bool { return model.Functions[i].Name < model.Functions[j].Name })
	return model, nil
}

func typeModel(fset *token.FileSet, decl *types.TypeName) TypeModel {
	model := TypeModel{Name: decl.Name(), Alias: decl.IsAlias(), Position: objectPosition(fset, decl), Supported: true}
	var underlying types.Type = decl.Type()
	if named, ok := decl.Type().(*types.Named); ok {
		underlying = named.Underlying()
	} else if alias, ok := decl.Type().(*types.Alias); ok {
		underlying = types.Unalias(alias)
	}
	switch value := underlying.(type) {
	case *types.Basic:
		model.Kind = "scalar"
		model.Underlying, model.Supported, model.UnsupportedReason = projectType(value)
	case *types.Struct:
		model.Kind = "struct"
		for index := 0; index < value.NumFields(); index++ {
			field := value.Field(index)
			if !field.Exported() {
				model.Supported = false
				model.UnsupportedReason = fmt.Sprintf("struct field %s is unexported", field.Name())
				continue
			}
			ref, supported, reason := projectType(field.Type())
			if !supported {
				model.Supported = false
				model.UnsupportedReason = fmt.Sprintf("struct field %s: %s", field.Name(), reason)
			}
			model.Fields = append(model.Fields, FieldModel{Name: field.Name(), Type: ref})
		}
	default:
		model.Kind = "unsupported"
		model.Supported = false
		model.UnsupportedReason = fmt.Sprintf("underlying type %s is outside bounded OctGo", types.TypeString(underlying, qualifier))
	}
	return model
}

func constantModel(fset *token.FileSet, decl *types.Const) ConstantModel {
	ref, supported, reason := projectType(decl.Type())
	model := ConstantModel{
		Name: decl.Name(), Type: ref, Value: decl.Val().ExactString(), Position: objectPosition(fset, decl),
		Supported: supported, UnsupportedReason: reason,
	}
	if supported {
		model.OctLiteral, model.Supported, model.UnsupportedReason = octConstantLiteral(decl.Val(), ref)
	}
	return model
}

func functionModel(fset *token.FileSet, decl *types.Func) FunctionModel {
	sig, ok := decl.Type().(*types.Signature)
	model := FunctionModel{Name: decl.Name(), Position: objectPosition(fset, decl), Signature: types.TypeString(decl.Type(), qualifier), Supported: ok}
	if !ok {
		model.UnsupportedReason = "declaration is not a function signature"
		return model
	}
	if sig.Recv() != nil {
		model.Supported = false
		model.UnsupportedReason = "methods are outside bounded OctGo"
		return model
	}
	if sig.TypeParams() != nil && sig.TypeParams().Len() != 0 {
		model.Supported = false
		model.UnsupportedReason = "generic functions are outside bounded OctGo"
	}
	model.Variadic = sig.Variadic()
	if sig.Variadic() {
		model.Supported = false
		model.UnsupportedReason = "variadic functions are outside bounded OctGo"
	}
	for index := 0; index < sig.Params().Len(); index++ {
		ref, supported, reason := projectType(sig.Params().At(index).Type())
		model.Parameters = append(model.Parameters, ref)
		if !supported && model.UnsupportedReason == "" {
			model.Supported = false
			model.UnsupportedReason = fmt.Sprintf("parameter %d: %s", index+1, reason)
		}
	}
	for index := 0; index < sig.Results().Len(); index++ {
		ref, supported, reason := projectType(sig.Results().At(index).Type())
		model.Results = append(model.Results, ref)
		if !supported && model.UnsupportedReason == "" {
			model.Supported = false
			model.UnsupportedReason = fmt.Sprintf("result %d: %s", index+1, reason)
		}
	}
	if len(model.Results) > 1 {
		model.Supported = false
		model.UnsupportedReason = "multiple results are outside bounded OctGo"
	}
	return model
}

func projectType(typ types.Type) (TypeRef, bool, string) {
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj()
		underlying, supported, reason := projectType(named.Underlying())
		if !supported || underlying.Kind == "record" {
			return TypeRef{Name: obj.Name(), Package: obj.Pkg().Path(), Kind: underlying.Kind}, false, reason
		}
		return TypeRef{Name: obj.Name(), Package: obj.Pkg().Path(), Kind: underlying.Kind}, true, ""
	}
	if alias, ok := typ.(*types.Alias); ok {
		obj := alias.Obj()
		underlying, supported, reason := projectType(types.Unalias(alias))
		return TypeRef{Name: obj.Name(), Package: obj.Pkg().Path(), Kind: underlying.Kind}, supported, reason
	}
	basic, ok := typ.(*types.Basic)
	if !ok {
		return TypeRef{Kind: "unsupported"}, false, fmt.Sprintf("type %s is outside bounded OctGo", types.TypeString(typ, qualifier))
	}
	switch basic.Kind() {
	case types.Bool:
		return TypeRef{Kind: "Bool"}, true, ""
	case types.Int:
		return TypeRef{Kind: "Int"}, true, ""
	case types.Float64:
		return TypeRef{Kind: "Float"}, true, ""
	case types.String:
		return TypeRef{Kind: "String"}, true, ""
	default:
		return TypeRef{Kind: "unsupported"}, false, fmt.Sprintf("Go primitive %s has no honest bounded OctGo mapping", basic.Name())
	}
}

func octConstantLiteral(value constant.Value, ref TypeRef) (string, bool, string) {
	switch value.Kind() {
	case constant.Bool:
		return strconv.FormatBool(constant.BoolVal(value)), true, ""
	case constant.Int:
		if _, ok := constant.Int64Val(value); !ok {
			return "", false, "integer constant is outside signed 64-bit Oct Int"
		}
		return value.ExactString(), true, ""
	case constant.String:
		return strconv.Quote(constant.StringVal(value)), true, ""
	case constant.Float:
		text := value.String()
		if strings.Contains(text, "/") {
			return "", false, "non-decimal exact Go float constant is outside bounded OctGo"
		}
		return text, true, ""
	default:
		return "", false, fmt.Sprintf("constant kind %s is outside bounded OctGo", value.Kind())
	}
}

func objectPosition(fset *token.FileSet, obj types.Object) Position {
	position := fset.Position(obj.Pos())
	return Position{File: position.Filename, Line: position.Line, Column: position.Column}
}

func qualifier(pkg *types.Package) string { return pkg.Path() }

func (m PackageModel) FindType(name string) (TypeModel, bool) {
	index := sort.Search(len(m.Types), func(i int) bool { return m.Types[i].Name >= name })
	returnValue := TypeModel{}
	if index < len(m.Types) && m.Types[index].Name == name {
		returnValue = m.Types[index]
		return returnValue, true
	}
	return returnValue, false
}

func (m PackageModel) FindFunction(name string) (FunctionModel, bool) {
	index := sort.Search(len(m.Functions), func(i int) bool { return m.Functions[i].Name >= name })
	if index < len(m.Functions) && m.Functions[index].Name == name {
		return m.Functions[index], true
	}
	return FunctionModel{}, false
}
