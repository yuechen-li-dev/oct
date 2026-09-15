package document

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type PDFResult struct {
	Bytes         []byte
	Engine        string
	EngineVersion string
}

func PDF(bundle LatexBundle) (PDFResult, error) {
	engine, err := latexEngine()
	if err != nil {
		return PDFResult{}, err
	}
	dir, err := os.MkdirTemp("", "octcument-latex-")
	if err != nil {
		return PDFResult{}, fmt.Errorf("Document.Pdf: create compilation directory: %w", err)
	}
	defer os.RemoveAll(dir)
	texPath := filepath.Join(dir, "paper.tex")
	if err := os.WriteFile(texPath, bundle.Source, 0o644); err != nil {
		return PDFResult{}, fmt.Errorf("Document.Pdf: write staged paper.tex: %w", err)
	}
	for _, file := range bundle.Files {
		path := filepath.Join(dir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return PDFResult{}, fmt.Errorf("Document.Pdf: stage %s: %w", file.Path, err)
		}
		if err := os.WriteFile(path, file.Bytes, 0o644); err != nil {
			return PDFResult{}, fmt.Errorf("Document.Pdf: stage %s: %w", file.Path, err)
		}
	}
	version := latexEngineVersion(engine)
	if err := runLatex(engine, dir); err != nil {
		return PDFResult{}, err
	}
	if bytes.Contains(readOptional(filepath.Join(dir, "paper.aux")), []byte(`\bibdata`)) {
		bibtex, lookErr := exec.LookPath("bibtex")
		if lookErr != nil {
			sibling := filepath.Join(filepath.Dir(engine), "bibtex")
			if runtime.GOOS == "windows" {
				sibling += ".exe"
			}
			if _, statErr := os.Stat(sibling); statErr == nil {
				bibtex, lookErr = sibling, nil
			}
		}
		if lookErr != nil {
			return PDFResult{}, fmt.Errorf("Document.Pdf: engine=%s bibliography requires bibtex: %w; source=%s", filepath.Base(engine), lookErr, texPath)
		}
		bibArgs := []string{"paper"}
		if strings.Contains(strings.ToLower(latexEngineVersion(bibtex)), "miktex") {
			bibArgs = append([]string{"--enable-installer"}, bibArgs...)
		}
		if err := runTool(bibtex, bibArgs, dir, "bibliography"); err != nil {
			return PDFResult{}, err
		}
	}
	if err := runLatex(engine, dir); err != nil {
		return PDFResult{}, err
	}
	if err := runLatex(engine, dir); err != nil {
		return PDFResult{}, err
	}
	payload, err := os.ReadFile(filepath.Join(dir, "paper.pdf"))
	if err != nil {
		return PDFResult{}, fmt.Errorf("Document.Pdf: engine=%s did not produce paper.pdf: %w; source=%s", filepath.Base(engine), err, texPath)
	}
	return PDFResult{Bytes: payload, Engine: filepath.Base(engine), EngineVersion: version}, nil
}

func latexEngine() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("OCT_LATEX_ENGINE")); configured != "" {
		path, err := exec.LookPath(configured)
		if err != nil {
			return "", fmt.Errorf("Document.Pdf: configured LaTeX engine %q was not found: %w", configured, err)
		}
		return path, nil
	}
	for _, candidate := range []string{"pdflatex", "xelatex", "lualatex"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("Document.Pdf: no LaTeX engine found (tried pdflatex, xelatex, lualatex); paper.tex remains the portable authoritative artifact")
}

func latexEngineVersion(engine string) string {
	cmd := exec.Command(engine, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unavailable"
	}
	line := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")[0]
	return strings.TrimSpace(line)
}

func runLatex(engine, dir string) error {
	args := []string{"-interaction=nonstopmode", "-halt-on-error", "-file-line-error", "-no-shell-escape", "-jobname=paper", "paper.tex"}
	if strings.Contains(strings.ToLower(latexEngineVersion(engine)), "miktex") {
		args = append([]string{"--enable-installer"}, args...)
	}
	return runTool(engine, args, dir, "LaTeX")
}

func runTool(program string, args []string, dir, phase string) error {
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=946684800", "FORCE_SOURCE_DATE=1", "TZ=UTC")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	exitCode := -1
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	return fmt.Errorf("Document.Pdf: %s engine=%s exit=%d: %s; source=%s", phase, filepath.Base(program), exitCode, conciseLatexDiagnostic(output), filepath.Join(dir, "paper.tex"))
}

func conciseLatexDiagnostic(output []byte) string {
	text := strings.ReplaceAll(string(output), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	selected := make([]string, 0, 8)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "!") || strings.Contains(trimmed, "Error") || strings.Contains(trimmed, "paper.tex:") || strings.Contains(trimmed, "not found") {
			selected = append(selected, trimmed)
			if len(selected) == 8 {
				break
			}
		}
	}
	if len(selected) == 0 {
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				selected = append(selected, strings.TrimSpace(line))
				if len(selected) == 4 {
					break
				}
			}
		}
	}
	result := strings.Join(selected, " | ")
	if len(result) > 1200 {
		result = result[:1200] + "..."
	}
	return result
}

func readOptional(path string) []byte { payload, _ := os.ReadFile(path); return payload }
