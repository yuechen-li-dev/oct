package newpkg

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateNameAcceptsStrictPascalCase(t *testing.T) {
	valid := []string{"OpenCV", "SignalTools", "BrownNoiseKalman", "A1", "HTTP2Client"}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateName(name); err != nil {
				t.Fatalf("expected %q valid: %v", name, err)
			}
		})
	}
}

func TestValidateNameRejectsInvalidNames(t *testing.T) {
	invalid := []string{
		"", strings.Repeat("A", 81), "oct-opencv", "signal_tools", "openCV", "Open CV",
		"Open/CV", "Open\\CV", "Open.CV", "Open:CV", "Manifest", "Main", "String", "Int", "Float",
		"Bool", "Void", "Bytes", "Error", "Array", "Map", "Pkg", "Exp", "New", "Run", "Build",
		"Test", "Artifact", "Bench", "Fmt",
	}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			err := ValidateName(name)
			if err == nil {
				t.Fatalf("expected %q invalid", name)
			}
			if !strings.Contains(err.Error(), name) && name != "" && !strings.Contains(name, `\`) {
				t.Fatalf("expected invalid-name error to mention %q, got %v", name, err)
			}
		})
	}
}

func TestNameDerivations(t *testing.T) {
	cases := []struct {
		name  string
		snake string
		kebab string
	}{
		{name: "OpenCV", snake: "open_cv", kebab: "open-cv"},
		{name: "SignalTools", snake: "signal_tools", kebab: "signal-tools"},
		{name: "BrownNoiseKalman", snake: "brown_noise_kalman", kebab: "brown-noise-kalman"},
		{name: "HTTP2Client", snake: "http_2_client", kebab: "http-2-client"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SnakeName(tc.name); got != tc.snake {
				t.Fatalf("SnakeName(%q)=%q want %q", tc.name, got, tc.snake)
			}
			if got := KebabName(tc.name); got != tc.kebab {
				t.Fatalf("KebabName(%q)=%q want %q", tc.name, got, tc.kebab)
			}
		})
	}
}

func TestPlanFilePathSets(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "library",
			opts: Options{Kind: KindLibrary, Name: "SignalTools", Dir: "SignalTools"},
			want: []string{"README.md", "SignalTools.Core.oct", "SignalTools.Core.octest", "manifest.oct"},
		},
		{
			name: "experiment",
			opts: Options{Kind: KindExperiment, Name: "BrownNoiseKalman", Dir: "BrownNoiseKalman"},
			want: []string{"M0/brown_noise_kalman_m0.oct", "M0/brown_noise_kalman_m0.octest", "README.md", "REPORT.md", "manifest.oct"},
		},
		{
			name: "application",
			opts: Options{Kind: KindApplication, Name: "MyApp", Dir: "MyApp"},
			want: []string{"Main.oct", "Main.octest", "README.md", "manifest.oct"},
		},
		{
			name: "app alias",
			opts: Options{Kind: KindApp, Name: "MyApp", Dir: "MyApp"},
			want: []string{"Main.oct", "Main.octest", "README.md", "manifest.oct"},
		},
		{
			name: "wrapper-library",
			opts: Options{Kind: KindWrapperLibrary, Name: "OpenCV", Dir: "OpenCV"},
			want: []string{"OpenCV.Core.oct", "OpenCV.Core.octest", "README.md", "manifest.oct", "sidecars/octxiliary-open-cv/README.md", "sidecars/octxiliary-open-cv/go.mod", "sidecars/octxiliary-open-cv/main.go"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, err := Plan(tc.opts)
			if err != nil {
				t.Fatalf("Plan failed: %v", err)
			}
			var got []string
			seen := map[string]bool{}
			for _, file := range files {
				got = append(got, file.Path)
				if seen[file.Path] {
					t.Fatalf("duplicate path %q", file.Path)
				}
				seen[file.Path] = true
				assertFinalNewlineAndLF(t, file)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("paths got %#v want %#v", got, tc.want)
			}
		})
	}
}

func TestPlanApplicationContent(t *testing.T) {
	filesByPath := planMap(t, Options{Kind: KindApplication, Name: "MyApp", Dir: "MyApp"})
	for _, snippet := range []string{
		`Kind: "application"`,
		`Description: "Runnable Oct application."`,
	} {
		if !strings.Contains(filesByPath["manifest.oct"], snippet) {
			t.Fatalf("application manifest missing %q:\n%s", snippet, filesByPath["manifest.oct"])
		}
	}
	for _, snippet := range []string{`package MyApp`, `fn Greeting() -> String`, `fn Main() -> Int`, `Hello from MyApp`} {
		if !strings.Contains(filesByPath["Main.oct"], snippet) {
			t.Fatalf("application source missing %q:\n%s", snippet, filesByPath["Main.oct"])
		}
	}
}

func TestPlanRepresentativeContent(t *testing.T) {
	filesByPath := planMap(t, Options{Kind: KindWrapperLibrary, Name: "OpenCV", Dir: "OpenCV"})
	manifest := filesByPath["manifest.oct"]
	for _, snippet := range []string{
		"Name: \"OpenCV\"",
		"Kind: \"wrapper\"",
		"Name: \"open-cv\"",
		"SidecarCommand: \"octxiliary-open-cv\"",
		"GoModuleDir: \"sidecars/octxiliary-open-cv\"",
		"WrapperFunction { OctName: \"EchoStringRaw\" WireName: \"OpenCVEchoString\" Args: [\"String\"] Return: \"String\" Fallible: true }",
	} {
		if !strings.Contains(manifest, snippet) {
			t.Fatalf("manifest missing %q:\n%s", snippet, manifest)
		}
	}
	mainGo := filesByPath["sidecars/octxiliary-open-cv/main.go"]
	if !strings.Contains(mainGo, "dispatcher := octxiliary.NewDispatcher(\"OpenCV\")") || !strings.Contains(mainGo, "octxiliary.OkString(req.ID, text)") {
		t.Fatalf("sidecar main missing expected dispatcher content:\n%s", mainGo)
	}
}

func TestPlanDoesNotTouchFilesystem(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "SignalTools")
	if _, err := Plan(Options{Kind: KindLibrary, Name: "SignalTools", Dir: dir}); err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Plan touched filesystem, stat err=%v", err)
	}
}

func TestWriteFailsIfTargetExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "SignalTools")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	err := Write(Options{Kind: KindLibrary, Name: "SignalTools", Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected target exists error, got %v", err)
	}
}

func TestWriteCreatesExpectedFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "BrownNoiseKalman")
	if err := Write(Options{Kind: KindExperiment, Name: "BrownNoiseKalman", Dir: dir}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	for _, rel := range []string{"manifest.oct", "README.md", "REPORT.md", "M0/brown_noise_kalman_m0.oct", "M0/brown_noise_kalman_m0.octest"} {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			t.Fatalf("expected file %s, info=%v err=%v", rel, info, err)
		}
	}
}

func TestWriteRejectsUnsafeTargetDir(t *testing.T) {
	for _, dir := range []string{"", "."} {
		t.Run(dir, func(t *testing.T) {
			err := Write(Options{Kind: KindLibrary, Name: "SignalTools", Dir: dir})
			if err == nil {
				t.Fatalf("expected unsafe target dir failure")
			}
		})
	}
}

func planMap(t *testing.T, opts Options) map[string]string {
	t.Helper()
	files, err := Plan(opts)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	result := map[string]string{}
	for _, file := range files {
		result[file.Path] = file.Content
	}
	return result
}

func assertFinalNewlineAndLF(t *testing.T, file File) {
	t.Helper()
	if !strings.HasSuffix(file.Content, "\n") {
		t.Fatalf("%s missing final newline", file.Path)
	}
	if strings.Contains(file.Content, "\r") {
		t.Fatalf("%s contains CR", file.Path)
	}
}
