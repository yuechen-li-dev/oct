package tester

import (
	"path/filepath"
	"testing"
)

func TestCompiledHarnessGroupingContract(t *testing.T) {
	tests := []testCase{
		{pkg: "Alpha", filePath: filepath.Join("Alpha", "one.octest"), displayName: "One", suites: []string{"Numerics"}},
		{pkg: "Alpha", filePath: filepath.Join("Alpha", "two.octest"), displayName: "Two", suites: []string{"Numerics"}},
		{pkg: "Beta", filePath: filepath.Join("Beta", "one.octest"), displayName: "One", suites: []string{"Numerics"}},
		{pkg: "Alpha", filePath: filepath.Join("Alpha", "plain.octest"), displayName: "Plain"},
		{pkg: "Alpha", filePath: filepath.Join("Alpha", "theory.octest"), displayName: "Rows[0]", caseIndex: 0},
		{pkg: "Alpha", filePath: filepath.Join("Alpha", "theory.octest"), displayName: "Rows[1]", caseIndex: 1},
		{pkg: "Alpha", filePath: filepath.Join("Alpha", "theory.octest"), displayName: "Rows[2]", caseIndex: 2},
	}
	groups := groupCompiledTestCases(tests)
	if len(groups) != 4 {
		t.Fatalf("groups = %d, want 4: %#v", len(groups), groups)
	}
	wantCases := map[string]int{
		"suite:Alpha:Numerics": 2,
		"suite:Beta:Numerics":  1,
		"file:Alpha:" + filepath.Clean(filepath.Join("Alpha", "plain.octest")):  1,
		"file:Alpha:" + filepath.Clean(filepath.Join("Alpha", "theory.octest")): 3,
	}
	for _, group := range groups {
		if got, ok := wantCases[group.id]; !ok || got != len(group.testCase) {
			t.Fatalf("unexpected group %q with %d cases", group.id, len(group.testCase))
		}
	}
}

func TestSelectedOctestFileDoesNotSelectSiblings(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "selected.octest")
	sources, err := selectedTestSources(selected)
	if err == nil {
		// selectedTestSources requires the path to exist; create and retry below.
		t.Fatal("expected missing selected path to fail")
	}
	writeTestFile(t, root, "selected.octest", "package Main\n[Fact]\nfn Selected() -> Void { Assert.True(true, \"selected\") }\n")
	writeTestFile(t, root, "sibling.octest", "package Main\n[Fact]\nfn Sibling() -> Void { Assert.True(true, \"sibling\") }\n")
	sources, err = selectedTestSources(selected)
	if err != nil {
		t.Fatal(err)
	}
	if !isSelectedSource(sources, selected) {
		t.Fatal("selected file was excluded")
	}
	if isSelectedSource(sources, filepath.Join(root, "sibling.octest")) {
		t.Fatal("sibling file was included")
	}
}
