package makecmd

// The native lowering deliberately emits ordinary direct-backend commands.
// This keeps state, trace, failure evidence, and staleness in one mechanism;
// it is not a shell build language.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/interpret"
)

type NativeTarget struct {
	Name, Variant, Kind, Language, Standard, Profile, Output, Manifest, ManifestSection string
	Sources, Inputs, Deps, IncludeDirs, Defines, LinkLibraries, InstallCopies           []string
}

func nativeTargets(v interpret.Value) []NativeTarget {
	if v.Kind != interpret.ValueRecord {
		return nil
	}
	v = v.Record.Fields["Targets"]
	if v.Kind != interpret.ValueArray {
		return nil
	}
	out := make([]NativeTarget, 0, len(v.Array))
	for _, e := range v.Array {
		if e.Kind != interpret.ValueRecord {
			continue
		}
		f := e.Record.Fields
		t := NativeTarget{Name: str(f, "Name"), Variant: str(f, "Variant"), Kind: enum(f, "Kind"), Language: enum(f, "Language"), Standard: str(f, "Standard"), Profile: enum(f, "Profile"), Output: str(f, "Output"), Manifest: str(f, "Manifest"), ManifestSection: str(f, "ManifestSection"), Sources: arr(f["Sources"]), Inputs: arr(f["Inputs"]), Deps: arr(f["Deps"]), IncludeDirs: arr(f["IncludeDirs"]), Defines: arr(f["Defines"]), LinkLibraries: arr(f["LinkLibraries"]), InstallCopies: arr(f["InstallCopies"])}
		out = append(out, t)
	}
	return out
}
func enum(m map[string]interpret.Value, k string) string {
	if v, ok := m[k]; ok && v.Kind == interpret.ValueEnum {
		return v.Enum.Variant
	}
	return ""
}

func lowerNative(root string, ns []NativeTarget) ([]CommandTarget, []PhonyTarget, error) {
	if len(ns) == 0 {
		return nil, nil, nil
	}
	cc, cxx, err := nativeTools()
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]NativeTarget{}
	for _, n := range ns {
		if n.Manifest != "" {
			sources, manifestErr := manifestSources(root, n.Manifest, n.ManifestSection)
			if manifestErr != nil {
				return nil, nil, fmt.Errorf("native target %q: %w", n.Name, manifestErr)
			}
			n.Sources = append(n.Sources, sources...)
			n.Inputs = append(n.Inputs, n.Manifest)
		}
		if n.Name == "" || n.Variant == "" {
			return nil, nil, fmt.Errorf("native target requires Name and Variant")
		}
		if _, ok := byName[n.Name]; ok {
			return nil, nil, fmt.Errorf("native target %q: duplicate", n.Name)
		}
		byName[n.Name] = n
	}
	commands := []CommandTarget{}
	phonies := []PhonyTarget{}
	objects := map[string][]string{}
	linkTarget := map[string]string{}
	for _, n := range ns {
		if n.Kind != "ObjectSet" && n.Kind != "SharedLibrary" && n.Kind != "Executable" {
			return nil, nil, fmt.Errorf("native target %q: unsupported Kind %q", n.Name, n.Kind)
		}
		if n.Language != "C" && n.Language != "Cpp" {
			return nil, nil, fmt.Errorf("native target %q: Language must be C or Cpp", n.Name)
		}
		if n.Kind == "ObjectSet" && n.Output != "" {
			return nil, nil, fmt.Errorf("native target %q: ObjectSet must not declare Output", n.Name)
		}
		compiler := cc
		if n.Language == "Cpp" {
			compiler = cxx
		}
		for _, src := range n.Sources {
			id := nativeID(n.Name, n.Variant, src)
			ext := ".o"
			if runtime.GOOS == "windows" {
				ext = ".obj"
			}
			obj := filepath.ToSlash(filepath.Join("out", "octmake", "native", sanitizeTargetName(n.Name), sanitizeTargetName(n.Variant), "obj", id+ext))
			args := nativeCompileArgs(n, src, obj)
			name := n.Name + ".compile." + id
			commands = append(commands, CommandTarget{Name: name, Inputs: append([]string{src}, n.Inputs...), Outputs: []string{obj}, Program: compiler, Args: args})
			objects[n.Name] = append(objects[n.Name], obj)
		}
		if n.Kind == "ObjectSet" {
			phonies = append(phonies, PhonyTarget{Name: n.Name, Deps: compileNames(commands, n.Name)})
			continue
		}
		var depObjs []string
		var depNames []string
		for _, d := range n.Deps {
			depObjs = append(depObjs, objects[d]...)
			depNames = append(depNames, compileNames(commands, d)...)
			if byName[d].Kind != "ObjectSet" {
				depNames = append(depNames, d)
			}
		}
		allObjs := append(append([]string{}, objects[n.Name]...), depObjs...)
		linkName := n.Name
		commands = append(commands, CommandTarget{Name: linkName, Inputs: append(append([]string{}, allObjs...), n.Inputs...), Outputs: []string{n.Output}, Deps: append(compileNames(commands, n.Name), depNames...), Program: compiler, Args: nativeLinkArgs(n, allObjs)})
		linkTarget[n.Name] = linkName
		for i, dst := range n.InstallCopies {
			commands = append(commands, CommandTarget{Name: fmt.Sprintf("%s.install.%d", n.Name, i), Inputs: []string{n.Output}, Outputs: []string{dst}, Deps: []string{linkName}, Program: nativeCopyProgram(), Args: nativeCopyArgs(n.Output, dst)})
		}
	}
	_ = linkTarget
	return commands, phonies, nil
}

func manifestSources(root, path, section string) ([]string, error) {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, path)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	var m struct {
		Production []string `json:"production_sources"`
		Tests      []string `json:"native_test_sources"`
		Slow       []string `json:"slow_only_test_sources"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse native manifest: %w", err)
	}
	var sources []string
	switch section {
	case "production_sources":
		sources = m.Production
	case "native_test_sources":
		sources = m.Tests
	case "slow_only_test_sources":
		sources = m.Slow
	default:
		return nil, fmt.Errorf("native manifest section %q is missing", section)
	}
	seen := map[string]bool{}
	for _, s := range sources {
		if seen[s] {
			return nil, fmt.Errorf("native manifest section %q duplicates %q", section, s)
		}
		seen[s] = true
		if !strings.HasSuffix(s, ".c") && !strings.HasSuffix(s, ".cpp") {
			return nil, fmt.Errorf("native manifest section %q has non-source %q", section, s)
		}
	}
	base := filepath.ToSlash(filepath.Dir(path))
	out := make([]string, 0, len(sources))
	for _, s := range sources {
		out = append(out, filepath.ToSlash(filepath.Join(base, s)))
	}
	return out, nil
}
func compileNames(cs []CommandTarget, prefix string) []string {
	out := []string{}
	for _, c := range cs {
		if strings.HasPrefix(c.Name, prefix+".compile.") {
			out = append(out, c.Name)
		}
	}
	return out
}
func nativeID(target, variant, source string) string {
	h := sha256.Sum256([]byte(target + "\x00" + variant + "\x00" + filepath.ToSlash(filepath.Clean(source))))
	return sanitizeTargetName(strings.TrimSuffix(filepath.ToSlash(filepath.Clean(source)), filepath.Ext(source))) + "-" + hex.EncodeToString(h[:6])
}
func nativeTools() (string, string, error) {
	if runtime.GOOS == "windows" {
		cc := os.Getenv("PROMETHEUS_NATIVE_CC")
		if cc == "" {
			cc = "cl.exe"
		}
		p, e := execPath(cc)
		if e != nil {
			return "", "", fmt.Errorf("native MSVC toolchain unavailable: %w; start a VS x64 developer shell or set PROMETHEUS_NATIVE_CC", e)
		}
		return p, p, nil
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	cxx := os.Getenv("CXX")
	if cxx == "" {
		cxx = "c++"
	}
	a, e := execPath(cc)
	if e != nil {
		return "", "", fmt.Errorf("native C toolchain unavailable: %w", e)
	}
	b, e := execPath(cxx)
	if e != nil {
		return "", "", fmt.Errorf("native C++ toolchain unavailable: %w", e)
	}
	return a, b, nil
}
func execPath(n string) (string, error) {
	if filepath.IsAbs(n) {
		if _, e := os.Stat(n); e != nil {
			return "", e
		}
		return n, nil
	}
	return exec.LookPath(n)
}
func nativeCompileArgs(n NativeTarget, src, obj string) []string {
	a := []string{}
	if runtime.GOOS == "windows" {
		a = []string{"/nologo", "/c", src, "/Fo" + obj}
		if n.Language == "C" {
			a = append(a, "/TC")
		} else {
			a = append(a, "/TP", "/EHsc")
		}
		if n.Profile == "Debug" {
			a = append(a, "/Od", "/Zi")
		} else {
			a = append(a, "/O2")
		}
		if n.Standard != "" {
			a = append(a, "/std:"+n.Standard)
		}
		for _, x := range n.IncludeDirs {
			a = append(a, "/I"+x)
		}
		for _, x := range n.Defines {
			a = append(a, "/D"+x)
		}
		return a
	}
	a = []string{"-c", src, "-o", obj, "-fPIC"}
	if n.Standard != "" {
		a = append(a, "-std="+n.Standard)
	}
	if n.Profile == "Debug" {
		a = append(a, "-O0", "-g")
	} else {
		a = append(a, "-O2")
	}
	for _, x := range n.IncludeDirs {
		a = append(a, "-I"+x)
	}
	for _, x := range n.Defines {
		a = append(a, "-D"+x)
	}
	return a
}
func nativeLinkArgs(n NativeTarget, objs []string) []string {
	a := append([]string{}, objs...)
	if runtime.GOOS == "windows" {
		a = append(a, "/link", "/OUT:"+n.Output)
		if n.Kind == "SharedLibrary" {
			a = append(a, "/DLL")
		}
		for _, l := range n.LinkLibraries {
			a = append(a, l)
		}
		return a
	}
	if n.Kind == "SharedLibrary" {
		a = append(a, "-shared")
	}
	for _, l := range n.LinkLibraries {
		if strings.HasPrefix(l, "-") {
			a = append(a, l)
		} else {
			a = append(a, "-l"+l)
		}
	}
	return append(a, "-o", n.Output)
}
func nativeCopyProgram() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "cp"
}
func nativeCopyArgs(s, d string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/c", "copy", "/Y", s, d}
	}
	return []string{s, d}
}
