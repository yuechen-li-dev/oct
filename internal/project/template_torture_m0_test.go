package project_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

const templateTortureRoot = "../../Language/Types/TemplateTortureM0"

func TestTemplateTortureM0ExplicitFileUsesSamePackageTemplateUniverse(t *testing.T) {
	program, err := project.LoadForTest(templateTortureRoot + "/valid/TemplateTortureValid/z_contracts.octest")
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	stats := program.Parametrics
	unique := stats.RecordInstantiations + stats.FunctionInstantiations + stats.FlowInstantiations
	if stats.TemplateRecords < 5 || stats.TemplateFunctions < 10 || stats.TemplateFlows < 2 {
		t.Fatalf("template collection did not see the package universe: %+v", stats)
	}
	if unique == 0 || stats.InstantiationRequests <= unique || stats.ReusedInstantiations == 0 {
		t.Fatalf("specialization reuse was not measured: unique=%d stats=%+v", unique, stats)
	}
	if stats.MaximumSpecializationDepth < 3 || stats.MaximumSpecializationDepth > 128 {
		t.Fatalf("unexpected specialization depth: %+v", stats)
	}
	for pkgName, pkg := range program.Packages {
		for _, decl := range pkg.Records {
			if decl.IsTemplate || len(decl.TypeParameters) != 0 {
				t.Fatalf("open record reached downstream package %s: %#v", pkgName, decl)
			}
		}
		for _, decl := range pkg.Functions {
			if decl.IsTemplate || len(decl.TypeParameters) != 0 {
				t.Fatalf("open function reached downstream package %s: %#v", pkgName, decl)
			}
		}
		for _, decl := range pkg.Flows {
			if decl.IsTemplate || len(decl.TypeParameters) != 0 {
				t.Fatalf("open flow reached downstream package %s: %#v", pkgName, decl)
			}
		}
	}
}

func TestTemplateTortureM0SpecializationIdentityIsDeterministic(t *testing.T) {
	first, err := project.LoadForTest(templateTortureRoot + "/valid/TemplateTortureValid")
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.LoadForTest(templateTortureRoot + "/valid/TemplateTortureValid")
	if err != nil {
		t.Fatal(err)
	}
	firstNames := concreteDeclarationNames(first)
	secondNames := concreteDeclarationNames(second)
	if !reflect.DeepEqual(firstNames, secondNames) {
		t.Fatalf("concrete declaration identity/order changed\nfirst:  %v\nsecond: %v", firstNames, secondNames)
	}
	first.Parametrics.ElaborationNanoseconds = 0
	second.Parametrics.ElaborationNanoseconds = 0
	if !reflect.DeepEqual(first.Parametrics, second.Parametrics) {
		t.Fatalf("parametric statistics changed: first=%+v second=%+v", first.Parametrics, second.Parametrics)
	}
}

func TestTemplateTortureM0CrossFileErrorRetainsInstantiationChain(t *testing.T) {
	program, err := project.LoadForTest(templateTortureRoot + "/provenance/use.octest")
	if err != nil {
		t.Fatal(err)
	}
	err = typecheck.CheckProgram(program)
	if err == nil {
		t.Fatal("expected dimensional specialization to fail")
	}
	message := err.Error()
	for _, want := range []string{
		"TemplateTortureProvenance.CallsDimensionallyInvalid<Float<m>>",
		"TemplateTortureProvenance.DimensionallyInvalid<Float<m>>",
		"b_invalid.oct",
		"return is Float<m^2>",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("diagnostic %q does not contain %q", message, want)
		}
	}
}

func concreteDeclarationNames(program project.Program) []string {
	var names []string
	for pkgName, pkg := range program.Packages {
		for _, decl := range pkg.Records {
			if decl.TemplateOrigin != nil {
				names = append(names, pkgName+".record."+decl.Name)
			}
		}
		for _, decl := range pkg.Functions {
			if decl.TemplateOrigin != nil {
				names = append(names, pkgName+".function."+decl.Name)
			}
		}
		for _, decl := range pkg.Flows {
			if decl.TemplateOrigin != nil {
				names = append(names, pkgName+".flow."+decl.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}
