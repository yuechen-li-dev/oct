package makecmd

// The native lowering deliberately emits ordinary direct-backend commands.
// This keeps state, trace, failure evidence, and staleness in one mechanism;
// it is not a shell build language.

import (
	"bytes"
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

type nativeToolchain struct {
	cc, cxx, link, vulkanInclude, vulkanLib string
	env                                     []string
}

type NativeTarget struct {
	Name, Variant, Kind, Language, Standard, Profile, Output, Manifest, ManifestSection                 string
	Sources, Inputs, Deps, IncludeDirs, Defines, ProjectRootStringDefines, LinkLibraries, InstallCopies []string
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
		t := NativeTarget{Name: str(f, "Name"), Variant: str(f, "Variant"), Kind: enum(f, "Kind"), Language: enum(f, "Language"), Standard: str(f, "Standard"), Profile: enum(f, "Profile"), Output: str(f, "Output"), Manifest: str(f, "Manifest"), ManifestSection: str(f, "ManifestSection"), Sources: arr(f["Sources"]), Inputs: arr(f["Inputs"]), Deps: arr(f["Deps"]), IncludeDirs: arr(f["IncludeDirs"]), Defines: arr(f["Defines"]), ProjectRootStringDefines: arr(f["ProjectRootStringDefines"]), LinkLibraries: arr(f["LinkLibraries"]), InstallCopies: arr(f["InstallCopies"])}
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

func lowerNative(root string, ns []NativeTarget) ([]CommandTarget, []PhonyTarget, map[string]string, error) {
	if len(ns) == 0 {
		return nil, nil, nil, nil
	}
	tools, err := nativeTools()
	if err != nil {
		return nil, nil, nil, err
	}
	byName := map[string]NativeTarget{}
	for i, n := range ns {
		if n.Manifest == "" && n.ManifestSection != "" {
			for _, input := range n.Inputs {
				if filepath.Base(input) == "native_manifest.json" {
					n.Manifest = input
					break
				}
			}
		}
		if n.Manifest != "" {
			sources, manifestErr := manifestSources(root, n.Manifest, n.ManifestSection)
			if manifestErr != nil {
				return nil, nil, nil, fmt.Errorf("native target %q: %w", n.Name, manifestErr)
			}
			n.Sources = append(n.Sources, sources...)
			n.Inputs = append(n.Inputs, n.Manifest)
		}
		if n.Name == "" || n.Variant == "" {
			return nil, nil, nil, fmt.Errorf("native target requires Name and Variant")
		}
		if _, ok := byName[n.Name]; ok {
			return nil, nil, nil, fmt.Errorf("native target %q: duplicate", n.Name)
		}
		ns[i] = n
		byName[n.Name] = n
	}
	commands := []CommandTarget{}
	phonies := []PhonyTarget{}
	artifacts := map[string]string{}
	objects := map[string][]string{}
	linkTarget := map[string]string{}
	for _, n := range ns {
		if n.Kind != "ObjectSet" && n.Kind != "SharedLibrary" && n.Kind != "Executable" {
			return nil, nil, nil, fmt.Errorf("native target %q: unsupported Kind %q", n.Name, n.Kind)
		}
		if n.Language != "C" && n.Language != "Cpp" {
			return nil, nil, nil, fmt.Errorf("native target %q: Language must be C or Cpp", n.Name)
		}
		if n.Kind == "ObjectSet" && n.Output != "" {
			return nil, nil, nil, fmt.Errorf("native target %q: ObjectSet must not declare Output", n.Name)
		}
		compiler := tools.cc
		if n.Language == "Cpp" {
			compiler = tools.cxx
		}
		for _, src := range n.Sources {
			id := nativeID(n.Name, n.Variant, src)
			ext := ".o"
			if runtime.GOOS == "windows" {
				ext = ".obj"
			}
			obj := filepath.ToSlash(filepath.Join("out", "octmake", "native", sanitizeTargetName(n.Name), sanitizeTargetName(n.Variant), "obj", id+ext))
			configured := n
			repo, _ := filepath.Abs(filepath.Join(root, "..", ".."))
			for _, name := range configured.ProjectRootStringDefines {
				configured.Defines = append(configured.Defines, name+`="`+escapeCString(filepath.ToSlash(repo))+`"`)
			}
			if tools.vulkanInclude != "" {
				configured.IncludeDirs = append(configured.IncludeDirs, tools.vulkanInclude)
			}
			args := nativeCompileArgs(configured, src, obj)
			name := n.Name + ".compile." + id
			command := CommandTarget{Name: name, Inputs: append([]string{src}, n.Inputs...), Outputs: []string{obj}, Program: compiler, Args: args, Env: tools.env}
			command.Discovery = nativeCompileDiscovery(src, obj, command.Args)
			commands = append(commands, command)
			objects[n.Name] = append(objects[n.Name], obj)
		}
		if n.Kind == "ObjectSet" {
			phonies = append(phonies, PhonyTarget{Name: n.Name, Deps: compileNames(commands, n.Name)})
			continue
		}
		logicalOutput := n.Output
		if runtime.GOOS == "windows" && n.Kind == "Executable" && !strings.EqualFold(filepath.Ext(n.Output), ".exe") {
			n.Output += ".exe"
		}
		if logicalOutput != n.Output {
			artifacts[logicalOutput] = n.Output
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
		configured := n
		if tools.vulkanLib != "" {
			configured.LinkLibraries = append(configured.LinkLibraries, "/LIBPATH:"+tools.vulkanLib)
		}
		commands = append(commands, CommandTarget{Name: linkName, Inputs: append(append([]string{}, allObjs...), n.Inputs...), Outputs: []string{n.Output}, Deps: append(compileNames(commands, n.Name), depNames...), Program: compiler, Args: nativeLinkArgs(configured, allObjs), Env: tools.env})
		linkTarget[n.Name] = linkName
		for i, dst := range n.InstallCopies {
			commands = append(commands, CommandTarget{Name: fmt.Sprintf("%s.install.%d", n.Name, i), Inputs: []string{n.Output}, Outputs: []string{dst}, Deps: []string{linkName}, Program: nativeCopyProgram(), Args: nativeCopyArgs(n.Output, dst)})
		}
	}
	_ = linkTarget
	return commands, phonies, artifacts, nil
}
func escapeCString(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
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
func nativeTools() (nativeToolchain, error) {
	if runtime.GOOS == "windows" {
		return discoverMSVC()
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
		return nativeToolchain{}, fmt.Errorf("native C toolchain unavailable: %w", e)
	}
	b, e := execPath(cxx)
	if e != nil {
		return nativeToolchain{}, fmt.Errorf("native C++ toolchain unavailable: %w", e)
	}
	return nativeToolchain{cc: a, cxx: b}, nil
}

func discoverMSVC() (nativeToolchain, error) {
	vswhere := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	if _, err := os.Stat(vswhere); err != nil {
		return nativeToolchain{}, fmt.Errorf("MSVC discovery: vswhere unavailable: %w", err)
	}
	out, err := exec.Command(vswhere, "-latest", "-products", "*", "-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "-property", "installationPath").Output()
	if err != nil {
		return nativeToolchain{}, fmt.Errorf("MSVC discovery via vswhere: %w", err)
	}
	install := strings.TrimSpace(string(out))
	if install == "" {
		return nativeToolchain{}, fmt.Errorf("MSVC discovery: no installation with x64 C++ tools")
	}
	dev := filepath.Join(install, "Common7", "Tools", "VsDevCmd.bat")
	dev, err = normalizeBatchPath(dev)
	if _, err := os.Stat(dev); err != nil {
		return nativeToolchain{}, fmt.Errorf("MSVC discovery: VsDevCmd missing: %w", err)
	}
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	all, bootErr := captureVsDevEnvironment(comspec, dev)
	if bootErr != nil {
		return nativeToolchain{}, fmt.Errorf("MSVC environment bootstrap: %w", bootErr)
	}
	path := all["PATH"]
	if path == "" || all["INCLUDE"] == "" || all["LIB"] == "" || all["LIBPATH"] == "" || all["VCTOOLSINSTALLDIR"] == "" || all["WINDOWSSDKDIR"] == "" || all["WINDOWSSDKVERSION"] == "" {
		return nativeToolchain{}, fmt.Errorf("MSVC environment bootstrap omitted required x64 toolchain variables")
	}
	lookup := func(name string) (string, error) {
		for _, d := range strings.Split(path, ";") {
			p := filepath.Join(d, name)
			if _, e := os.Stat(p); e == nil {
				return p, nil
			}
		}
		return "", fmt.Errorf("%s not found in captured MSVC PATH", name)
	}
	cl, err := lookup("cl.exe")
	if err != nil {
		return nativeToolchain{}, err
	}
	link, err := lookup("link.exe")
	if err != nil {
		return nativeToolchain{}, err
	}
	include := all["INCLUDE"]
	found := false
	for _, d := range strings.Split(include, ";") {
		if _, e := os.Stat(filepath.Join(d, "stdint.h")); e == nil {
			found = true
			break
		}
	}
	if !found {
		return nativeToolchain{}, fmt.Errorf("MSVC toolset include roots do not contain stdint.h")
	}
	vk := all["VULKAN_SDK"]
	if vk == "" {
		return nativeToolchain{}, fmt.Errorf("Vulkan SDK unavailable: VULKAN_SDK is empty")
	}
	if _, e := os.Stat(filepath.Join(vk, "Include", "vulkan", "vulkan.h")); e != nil {
		return nativeToolchain{}, fmt.Errorf("Vulkan header unavailable: %w", e)
	}
	if _, e := os.Stat(filepath.Join(vk, "Lib", "vulkan-1.lib")); e != nil {
		return nativeToolchain{}, fmt.Errorf("Vulkan loader library unavailable: %w", e)
	}
	keys := []string{"PATH", "INCLUDE", "LIB", "LIBPATH", "VCToolsInstallDir", "VCInstallDir", "WindowsSdkDir", "WindowsSDKVersion", "UniversalCRTSdkDir", "UCRTVersion", "VSCMD_ARG_HOST_ARCH", "VSCMD_ARG_TGT_ARCH", "VULKAN_SDK"}
	env := []string{}
	for _, k := range keys {
		if v := all[k]; v != "" {
			env = append(env, k+"="+v)
		}
	}
	if all["VSCMD_ARG_HOST_ARCH"] != "x64" || all["VSCMD_ARG_TGT_ARCH"] != "x64" {
		return nativeToolchain{}, fmt.Errorf("MSVC bootstrap target architecture %q, want x64", all["VSCMD_ARG_TGT_ARCH"])
	}
	return nativeToolchain{cc: cl, cxx: cl, link: link, env: env, vulkanInclude: filepath.Join(vk, "Include"), vulkanLib: filepath.Join(vk, "Lib")}, nil
}
func parseEnvironment(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] != "" {
			out[strings.ToUpper(parts[0])] = parts[1]
		}
	}
	return out
}
func normalizeBatchPath(path string) (string, error) {
	path = filepath.Clean(path)
	path = strings.TrimPrefix(path, `\\?\`)
	if strings.HasPrefix(path, `\\`) {
		return "", fmt.Errorf("unsupported UNC VsDevCmd path %q", path)
	}
	if strings.ContainsAny(path, "&|<>^") {
		return "", fmt.Errorf("unsafe VsDevCmd path %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}
func captureVsDevEnvironment(comspec, dev string) (map[string]string, error) {
	dir, err := os.MkdirTemp("", "oct msvc bootstrap ")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	name := "oct-msvc-bootstrap.cmd"
	content := "@echo off\r\ncall \"" + dev + "\" -no_logo -arch=x64 -host_arch=x64 >nul\r\nif errorlevel 1 exit /b %errorlevel%\r\nset\r\nexit /b 0\r\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
		return nil, err
	}
	cmd := exec.Command(comspec, "/d", "/q", "/c", name)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("exit %d; stderr: %s; stdout: %s", cmd.ProcessState.ExitCode(), bounded(stderr.String()), bounded(stdout.String()))
	}
	return parseEnvironment(stdout.String()), nil
}
func bounded(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1024 {
		return s[:1024]
	}
	return s
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
			standard := n.Standard
			if standard == "c++23" {
				standard = "c++latest"
			}
			a = append(a, "/std:"+standard)
		}
		for _, x := range n.IncludeDirs {
			a = append(a, "/I"+x)
		}
		for _, x := range n.Defines {
			a = append(a, "/D"+x)
		}
		// The executor supplies the following attempt-local filename from the
		// typed DiscoverySpec.  Keeping it out of this lowered command makes the
		// action hash stable while still giving cl.exe a fresh output per run.
		a = append(a, "/sourceDependencies")
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

func nativeCompileDiscovery(source, object string, args []string) *DiscoverySpec {
	if runtime.GOOS != "windows" {
		return nil
	}
	return &DiscoverySpec{
		Kind:          MSVCSourceDependenciesKind,
		SchemaVersion: MSVCSourceDependenciesSchemaV1,
		OutputArgument: RuntimeOutputArgument{
			Position: len(args),
		},
		ExpectedSourceIdentity:         source,
		ExpectedPhysicalOutputIdentity: object,
	}
}
func nativeLinkArgs(n NativeTarget, objs []string) []string {
	a := append([]string{}, objs...)
	if runtime.GOOS == "windows" {
		a = append(a, "/link", "/OUT:"+n.Output)
		if n.Kind == "SharedLibrary" {
			a = append(a, "/DLL")
		}
		for _, l := range n.LinkLibraries {
			if l == "vulkan" {
				a = append(a, "vulkan-1.lib")
			} else {
				a = append(a, l)
			}
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
