package newpkg

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type Kind string

const (
	DefaultManifestDate = "2026-06-15"

	KindLibrary        Kind = "library"
	KindExperiment     Kind = "experiment"
	KindWrapperLibrary Kind = "wrapper-library"
	KindApplication    Kind = "application"
	KindApp            Kind = "app"
)

type Options struct {
	Kind Kind
	Name string
	Dir  string
}

type File struct {
	Path    string
	Content string
}

var validNamePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

var reservedNames = map[string]string{
	"Manifest": "reserved manifest package name",
	"Main":     "reserved entry package name",
	"String":   "built-in scalar/type family name",
	"Int":      "built-in scalar/type family name",
	"Float":    "built-in scalar/type family name",
	"Bool":     "built-in scalar/type family name",
	"Void":     "built-in scalar/type family name",
	"Bytes":    "built-in scalar/type family name",
	"Error":    "built-in scalar/type family name",
	"Array":    "built-in scalar/type family name",
	"Map":      "built-in scalar/type family name",
	"Pkg":      "top-level command family name",
	"Exp":      "top-level command family name",
	"New":      "top-level command family name",
	"Run":      "top-level command family name",
	"Init":     "top-level command family name",
	"Build":    "top-level command family name",
	"Test":     "top-level command family name",
	"Artifact": "top-level command family name",
	"Bench":    "top-level command family name",
	"Fmt":      "top-level command family name",
}

func Plan(opts Options) ([]File, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	var files []File
	switch opts.Kind {
	case KindLibrary:
		files = libraryFiles(opts.Name)
	case KindApplication, KindApp:
		files = applicationFiles(opts.Name)
	case KindExperiment:
		files = experimentFiles(opts.Name)
	case KindWrapperLibrary:
		files = wrapperLibraryFiles(opts.Name)
	default:
		return nil, fmt.Errorf("unsupported package kind %q", opts.Kind)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func InitWrite(opts Options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}
	target := filepath.Clean(opts.Dir)
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("target directory %q does not exist", opts.Dir)
		}
		return fmt.Errorf("check target directory %q: %w", opts.Dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("target %q is not a directory", opts.Dir)
	}
	manifestPath := filepath.Join(target, "manifest.oct")
	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("manifest.oct already exists in %q; oct init refuses to overwrite existing manifests", opts.Dir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check manifest.oct in %q: %w", opts.Dir, err)
	}
	content, err := Manifest(opts.Kind, opts.Name)
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write manifest.oct in %q: %w", opts.Dir, err)
	}
	return nil
}

func Manifest(kind Kind, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	switch kind {
	case KindLibrary:
		return libraryManifest(name), nil
	case KindApplication, KindApp:
		return applicationManifest(name), nil
	case KindExperiment:
		return experimentManifest(name), nil
	case KindWrapperLibrary:
		return wrapperManifest(name, KebabName(name)), nil
	default:
		return "", fmt.Errorf("unsupported package kind %q", kind)
	}
}

func Write(opts Options) error {
	files, err := Plan(opts)
	if err != nil {
		return err
	}
	target := filepath.Clean(opts.Dir)
	if target == "." || target == string(filepath.Separator) {
		return fmt.Errorf("target directory %q is not a package scaffold directory", opts.Dir)
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("target directory %q already exists", opts.Dir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check target directory %q: %w", opts.Dir, err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create target directory %q: %w", opts.Dir, err)
	}
	createdTarget := true
	defer func() {
		if createdTarget {
			_ = os.RemoveAll(target)
		}
	}()

	for _, file := range files {
		path, err := safeTargetPath(target, file.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %q: %w", file.Path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return fmt.Errorf("write %q: %w", file.Path, err)
		}
	}
	createdTarget = false
	return nil
}

func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid package name %q: name must not be empty", name)
	}
	if len(name) > 80 {
		return fmt.Errorf("invalid package name %q: name must be at most 80 characters", name)
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return fmt.Errorf("invalid package name %q: whitespace is not allowed", name)
		}
	}
	if strings.ContainsAny(name, "-_/.:") || strings.ContainsAny(name, `\`) {
		return fmt.Errorf("invalid package name %q: use strict PascalCase letters and digits only", name)
	}
	if !validNamePattern.MatchString(name) {
		return fmt.Errorf("invalid package name %q: must match [A-Z][A-Za-z0-9]*", name)
	}
	if reason, ok := reservedNames[name]; ok {
		return fmt.Errorf("invalid package name %q: %s", name, reason)
	}
	return nil
}

func SnakeName(name string) string {
	return strings.ReplaceAll(KebabName(name), "-", "_")
}

func KebabName(name string) string {
	if name == "" {
		return ""
	}
	var out strings.Builder
	for i, r := range name {
		if i > 0 && startsWord(name, i) {
			out.WriteByte('-')
		}
		out.WriteRune(unicode.ToLower(r))
	}
	return out.String()
}

func validateOptions(opts Options) error {
	if err := ValidateName(opts.Name); err != nil {
		return err
	}
	if strings.TrimSpace(opts.Dir) == "" {
		return fmt.Errorf("target directory for package %q must not be empty", opts.Name)
	}
	return nil
}

func startsWord(name string, i int) bool {
	current := rune(name[i])
	previous := rune(name[i-1])
	if unicode.IsDigit(current) {
		return unicode.IsLetter(previous)
	}
	if !unicode.IsUpper(current) {
		return false
	}
	if unicode.IsLower(previous) || unicode.IsDigit(previous) {
		return true
	}
	if unicode.IsUpper(previous) && i+1 < len(name) {
		return unicode.IsLower(rune(name[i+1]))
	}
	return false
}

func safeTargetPath(target string, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("unsafe scaffold path %q", rel)
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == "." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) || cleanRel == ".." {
		return "", fmt.Errorf("unsafe scaffold path %q", rel)
	}
	path := filepath.Join(target, cleanRel)
	cleanTarget := filepath.Clean(target)
	if path != cleanTarget && !strings.HasPrefix(path, cleanTarget+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe scaffold path %q", rel)
	}
	return path, nil
}

func applicationFiles(name string) []File {
	return []File{
		{Path: "README.md", Content: applicationReadme(name)},
		{Path: "Main.oct", Content: applicationMain(name)},
		{Path: "Main.octest", Content: applicationMainTest(name)},
		{Path: "manifest.oct", Content: applicationManifest(name)},
	}
}

func libraryFiles(name string) []File {
	return []File{
		{Path: "README.md", Content: libraryReadme(name)},
		{Path: name + ".Core.oct", Content: libraryCore(name)},
		{Path: name + ".Core.octest", Content: libraryCoreTest(name)},
		{Path: "manifest.oct", Content: libraryManifest(name)},
	}
}

func experimentFiles(name string) []File {
	snake := SnakeName(name)
	return []File{
		{Path: "M0/" + snake + "_m0.oct", Content: experimentCore(name)},
		{Path: "M0/" + snake + "_m0.octest", Content: experimentCoreTest(name)},
		{Path: "README.md", Content: experimentReadme(name)},
		{Path: "REPORT.md", Content: experimentReport(name)},
		{Path: "manifest.oct", Content: experimentManifest(name)},
	}
}

func wrapperLibraryFiles(name string) []File {
	kebab := KebabName(name)
	sidecarDir := "sidecars/octxiliary-" + kebab + "/"
	return []File{
		{Path: "README.md", Content: wrapperReadme(name, kebab)},
		{Path: name + ".Core.oct", Content: wrapperCore(name)},
		{Path: name + ".Core.octest", Content: wrapperCoreTest(name)},
		{Path: "manifest.oct", Content: wrapperManifest(name, kebab)},
		{Path: sidecarDir + "README.md", Content: sidecarReadme(name, kebab)},
		{Path: sidecarDir + "go.mod", Content: sidecarGoMod(kebab)},
		{Path: sidecarDir + "main.go", Content: sidecarMain(name)},
	}
}

func applicationManifest(name string) string {
	return fmt.Sprintf(`package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Authors: String[]
    Date: String
    Kind: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: %q
        Version: "0.1.0"
        Description: "Runnable Oct application."
        Authors: ["Unknown"]
        Date: %q
        Kind: "application"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
`, name, DefaultManifestDate)
}

func applicationMain(name string) string {
	return fmt.Sprintf(`package %s

/// Return the scaffold greeting for this application.
fn Greeting() -> String {
    return %q
}

fn Main() -> Int {
    Print(Greeting())
    return 0
}
`, name, "Hello from "+name)
}

func applicationMainTest(name string) string {
	return fmt.Sprintf(`package %s

[Fact]
fn GreetingReturnsMessage() -> Void {
    Assert.Equal(%q, Greeting(), "greeting should match scaffold")
}
`, name, "Hello from "+name)
}

func applicationReadme(name string) string {
	return fmt.Sprintf(`# %s

Generated application scaffold by `+"`oct new application`"+`.

Application packages are runnable Oct programs, services, UIs, or CLIs. They are distinct from reusable libraries, experiments whose primary outputs are evidence/artifacts, and wrapper libraries that expose external sidecars.

APP1 records `+"`Kind: \"application\"`"+` and creates a runnable `+"`Main()`"+`; application packaging, deployment profiles, optional `+"`Make.oct`"+`, and UIBridge/Machina integration are future work.
`, name)
}

func libraryManifest(name string) string {
	return fmt.Sprintf(`package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Authors: String[]
    Date: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: %q
        Version: "0.1.0"
        Description: %q
        Authors: ["Unknown"]
        Date: %q
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
`, name, name+" package", DefaultManifestDate)
}

func libraryCore(name string) string {
	return fmt.Sprintf(`package %s

/// Return the input value unchanged.
fn Identity(value: Int) -> Int {
    return value
}
`, name)
}

func libraryCoreTest(name string) string {
	return fmt.Sprintf(`package %s

[Fact]
fn IdentityReturnsInput() -> Void {
    Assert.Equal(7, Identity(7), "identity should return the input")
}
`, name)
}

func libraryReadme(name string) string {
	return fmt.Sprintf(`# %s

Generated by `+"`oct new library`"+`.

Replace the sample `+"`Identity`"+` function and its test with package functionality.
`, name)
}

func experimentManifest(name string) string {
	return fmt.Sprintf(`package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Authors: String[]
    Date: String
    Kind: String
    EntryMilestone: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: %q
        Version: "0.1.0"
        Description: %q
        Authors: ["Unknown"]
        Date: %q
        Kind: "experiment"
        EntryMilestone: "M0"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
`, name, name+" experiment", DefaultManifestDate)
}

func experimentCore(name string) string {
	return fmt.Sprintf(`package %s

/// Return the starting sample count for the first experiment milestone.
fn M0SampleCount() -> Int {
    return 1
}
`, name)
}

func experimentCoreTest(name string) string {
	return fmt.Sprintf(`package %s

[Fact]
fn M0SampleCountIsPositive() -> Void {
    Assert.True(M0SampleCount() > 0, "M0 sample count should be positive")
}
`, name)
}

func experimentReadme(name string) string {
	return fmt.Sprintf(`# %s

Generated experiment scaffold by `+"`oct new experiment`"+`.

Start from the `+"`M0/`"+` milestone files and expand them as the experiment progresses.
`, name)
}

func experimentReport(name string) string {
	return fmt.Sprintf(`# %s Report

## M0

Record the first experiment milestone results here.
`, name)
}

func wrapperManifest(name string, kebab string) string {
	sidecarCommand := "octxiliary-" + kebab
	return fmt.Sprintf(`package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Authors: String[]
    Date: String
    Kind: String
    Dependencies: Dependency[]
    Wrappers: Wrapper[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

record Wrapper {
    Name: String
    Family: String
    Protocol: String
    SidecarCommand: String
    GoModuleDir: String
    Functions: WrapperFunction[]
}

record WrapperFunction {
    OctName: String
    WireName: String
    Args: String[]
    Return: String
    Fallible: Bool
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: %q
        Version: "0.1.0"
        Description: %q
        Authors: ["Unknown"]
        Date: %q
        Kind: "wrapper"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
        Wrappers: [
            Wrapper {
                Name: %q
                Family: %q
                Protocol: "octxiliary.v0"
                SidecarCommand: %q
                GoModuleDir: %q
                Functions: [
                    WrapperFunction { OctName: "EchoStringRaw" WireName: %q Args: ["String"] Return: "String" Fallible: true }
                ]
            }
        ]
    }
}
`, name, name+" wrapper package", DefaultManifestDate, kebab, name, sidecarCommand, "sidecars/"+sidecarCommand, name+"EchoString")
}

func wrapperCore(name string) string {
	return fmt.Sprintf(`package %s

/// Returns true when the generated package scaffold is loadable.
fn ScaffoldReady() -> Bool {
    return true
}
`, name)
}

func wrapperCoreTest(name string) string {
	return fmt.Sprintf(`package %s

[Fact]
fn ScaffoldLoads() -> Void {
    Assert.True(ScaffoldReady(), "wrapper scaffold should load before native sidecar build support")
}
`, name)
}

func wrapperReadme(name string, kebab string) string {
	return fmt.Sprintf(`# %s

Generated wrapper-library scaffold by `+"`oct new wrapper-library`"+`.

`+"`oct pkg wrappers`"+` can inspect the wrapper metadata in `+"`manifest.oct`"+`.

Native sidecar build and dispatch lifecycle support is future work. No native code was built or run by scaffolding.

The generated raw wrapper function metadata points at `+"`octxiliary-%s`"+`; update the sidecar module path and implementation before publishing.
`, name, kebab)
}

func sidecarGoMod(kebab string) string {
	return fmt.Sprintf(`module example.com/%s-sidecar

go 1.24.0

require github.com/yuechen-li-dev/oct v0.0.0
`, kebab)
}

func sidecarMain(name string) string {
	return fmt.Sprintf(`package main

import (
    "os"

    "github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

func main() {
    dispatcher := octxiliary.NewDispatcher(%q)
    dispatcher.HandleFunc(%q, func(req octxiliary.Request) octxiliary.Response {
        text, err := octxiliary.ArgString(req, 0)
        if err != nil {
            return octxiliary.Err(req.ID, err)
        }
        return octxiliary.OkString(req.ID, text)
    })
    os.Exit(octxiliary.Main(os.Stdin, os.Stdout, dispatcher.HandleRequest))
}
`, name, name+"EchoString")
}

func sidecarReadme(name string, kebab string) string {
	return fmt.Sprintf(`# octxiliary-%s

Scaffold-only sidecar for the %s wrapper package.

`+"`oct new wrapper-library`"+` does not build or run this sidecar. Update the Go module path and implementation before publishing the package.
`, kebab, name)
}
