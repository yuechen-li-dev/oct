package main

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestM22eAxialStiffnessMatrix(t *testing.T) {
    root := setupM22eStructuresFixture(t)
    writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
        "package Main",
        "import Structures",
        "",
        "fn Main() -> Int {",
        "    let element = Structures.BarElement2D {",
        "        Area: 0.02m*m",
        "        YoungsModulus: 200000000000kg/m/s/s",
        "        Length: 2m",
        "    }",
        "    let k = Structures.AxialStiffness(element)",
        "    let probe = k @ vector[1.0m, 0.0m]",
        "    if Abs(probe[0] - 2000000000kg*m/s^2) > 0.1kg*m/s^2 { return 1 }",
        "    if Abs(probe[1] + 2000000000kg*m/s^2) > 0.1kg*m/s^2 { return 2 }",
        "    if Abs(k[0, 0] - 2000000000kg/s^2) > 0.1kg/s^2 { return 3 }",
        "    if Abs(k[0, 1] + 2000000000kg/s^2) > 0.1kg/s^2 { return 4 }",
        "    return 0",
        "}",
    }, "\n"))

    stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
    if err != nil {
        t.Fatalf("run failed: %v stderr=%s", err, stderr)
    }
    if stdout != "0\n" {
        t.Fatalf("expected success code 0, got %q", stdout)
    }
}

func TestM22eMatrixVectorInternalForce(t *testing.T) {
    root := setupM22eStructuresFixture(t)
    writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
        "package Main",
        "import Structures",
        "",
        "fn Main() -> Int {",
        "    let element = Structures.BarElement2D {",
        "        Area: 0.02m*m",
        "        YoungsModulus: 200000000000kg/m/s/s",
        "        Length: 2m",
        "    }",
        "    let force = Structures.AxialForceFromDisplacement(element, vector[0.0m, 0.001m])",
        "    if Abs(force[0] + 2000000kg*m/s^2) > 0.1kg*m/s^2 { return 1 }",
        "    if Abs(force[1] - 2000000kg*m/s^2) > 0.1kg*m/s^2 { return 2 }",
        "    return 0",
        "}",
    }, "\n"))

    stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
    if err != nil {
        t.Fatalf("run failed: %v stderr=%s", err, stderr)
    }
    if stdout != "0\n" {
        t.Fatalf("expected success code 0, got %q", stdout)
    }
}

func TestM22ePackageIntegrationRunAndBuild(t *testing.T) {
    root := setupM22eFixture(t)
    entry := filepath.Join(root, "Main", "main.oct")

    stdout, stderr, err := executeCLI("run", entry)
    if err != nil {
        t.Fatalf("run failed: %v stderr=%s", err, stderr)
    }
    if !strings.Contains(stdout, "matrix[[2e+09kg/s^2") {
        t.Fatalf("expected stiffness matrix print output, got %q", stdout)
    }
    if !strings.Contains(stdout, "vector[-2e+06m*kg/s^2, 2e+06m*kg/s^2]") {
        t.Fatalf("expected internal force vector output, got %q", stdout)
    }

    buildStdout, buildStderr, buildErr := executeCLI("build", entry)
    if buildErr != nil {
        t.Fatalf("build failed: %v stdout=%s stderr=%s", buildErr, buildStdout, buildStderr)
    }
    if !strings.Contains(buildStdout, "build succeeded") {
        t.Fatalf("expected build success output, got %q", buildStdout)
    }
}

func TestM22eBuildFailureDoesNotEmitArtifact(t *testing.T) {
    root := setupM22eStructuresFixture(t)
    entry := filepath.Join(root, "Main", "main.oct")
    writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
        "package Main",
        "import Structures",
        "",
        "fn Main() -> Int {",
        "    let bad = Structures.BarElement2D {",
        "        Area: 0.02m*m",
        "        YoungsModulus: 200000000000kg/m/s/s",
        "        Length: 2s",
        "    }",
        "    Print(bad)",
        "    return 0",
        "}",
    }, "\n"))

    stdout, stderr, err := executeCLI("build", entry)
    if err == nil {
        t.Fatalf("expected build failure, got success with stdout %q", stdout)
    }
    if !strings.Contains(stderr, "expects Float<m>, got Int<s>") {
        t.Fatalf("unexpected stderr %q", stderr)
    }
    if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
        t.Fatalf("expected no artifact, stat err = %v", statErr)
    }
}

func setupM22eStructuresFixture(t *testing.T) string {
    t.Helper()
    root := t.TempDir()
    copyDir(t, filepath.Join("..", "..", "testdata", "m22e", "valid", "Structures"), filepath.Join(root, "Structures"))
    return root
}

func setupM22eFixture(t *testing.T) string {
    t.Helper()
    root := t.TempDir()
    copyDir(t, filepath.Join("..", "..", "testdata", "m22e", "valid", "Structures"), filepath.Join(root, "Structures"))
    copyDir(t, filepath.Join("..", "..", "testdata", "m22e", "valid", "Main"), filepath.Join(root, "Main"))
    return root
}
