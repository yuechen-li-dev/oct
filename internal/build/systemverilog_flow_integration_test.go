//go:build integration

package build

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVerilogM2IcarusAndYosysEvidence(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("repository qualification currently provides Icarus/Yosys through WSL")
	}
	if _, err := exec.LookPath("wsl"); err != nil {
		t.Skip("WSL is unavailable")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	wslRoot := "/mnt/" + strings.ToLower(root[:1]) + strings.ReplaceAll(filepath.ToSlash(root[2:]), " ", "\\ ")
	fixtures := []string{"basic_fsm", "remember_resume", "utility_policy"}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			golden := "Language/Profiles/VerilogM2/valid/" + fixture + "/" + fixture + ".golden.sv"
			testbench := "Language/Profiles/VerilogM2/valid/" + fixture + "/equivalence_tb.sv"
			top := fixture + "_tb"
			command := "cd " + wslRoot + " && iverilog -g2012 -s " + top + " -o /tmp/veril_oct_m2_" + fixture + ".vvp " + golden + " " + testbench + " && vvp /tmp/veril_oct_m2_" + fixture + ".vvp"
			output, err := exec.Command("wsl", "sh", "-lc", command).CombinedOutput()
			if err != nil {
				t.Fatalf("Icarus %s: %v\n%s", fixture, err, output)
			}
		})
	}
	tops := map[string]string{"basic_fsm": "Counter", "remember_resume": "Interruptible", "utility_policy": "UtilityController"}
	for _, fixture := range fixtures {
		yosys := "cd " + wslRoot + " && yosys -p 'read_verilog -sv Language/Profiles/VerilogM2/valid/" + fixture + "/" + fixture + ".golden.sv; hierarchy -check -top " + tops[fixture] + "; proc; opt; check; stat'"
		output, err := exec.Command("wsl", "sh", "-lc", yosys).CombinedOutput()
		if err != nil {
			t.Fatalf("Yosys %s: %v\n%s", fixture, err, output)
		}
		text := string(output)
		if !strings.Contains(text, "Found and reported 0 problems") || (!strings.Contains(text, "$sdff") && !strings.Contains(text, "$dff")) {
			t.Fatalf("Yosys %s output lacks clean sequential evidence:\n%s", fixture, text)
		}
	}
}
