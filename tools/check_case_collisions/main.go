// Command check_case_collisions rejects repository paths that differ only by case
// and verifies that tracked files can be assembled into a Go module ZIP.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

func main() {
	modulePath := flag.String("module", "github.com/yuechen-li-dev/oct", "module path used for ZIP validation")
	version := flag.String("version", "v0.0.0-hardenm0", "module version used for ZIP validation")
	outputPath := flag.String("output", "", "optional path for the validated module ZIP")
	flag.Parse()

	paths, err := trackedPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	collisions := caseInsensitiveCollisions(paths)
	if len(collisions) == 0 {
		if err := createModuleZip(paths, module.Version{Path: *modulePath, Version: *version}, *outputPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	for _, collision := range collisions {
		fmt.Fprintf(os.Stderr, "case-insensitive repository path collision: %s\n", strings.Join(collision, " | "))
	}
	os.Exit(1)
}

type repositoryFile string

func (file repositoryFile) Path() string { return string(file) }
func (file repositoryFile) Lstat() (os.FileInfo, error) {
	return os.Lstat(string(file))
}
func (file repositoryFile) Open() (io.ReadCloser, error) {
	return os.Open(string(file))
}

func createModuleZip(paths []string, version module.Version, outputPath string) error {
	files := make([]modzip.File, 0, len(paths))
	for _, path := range paths {
		files = append(files, repositoryFile(path))
	}
	var output io.Writer = io.Discard
	var destination *os.File
	if outputPath != "" {
		var err error
		destination, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create module ZIP output: %w", err)
		}
		defer destination.Close()
		output = destination
	}
	if err := modzip.Create(output, version, files); err != nil {
		return fmt.Errorf("create module ZIP: %w", err)
	}
	return nil
}

func trackedPaths() ([]string, error) {
	command := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked repository paths: %w", err)
	}
	paths := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Split(splitNUL)
	for scanner.Scan() {
		if scanner.Text() != "" {
			paths = append(paths, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan tracked repository paths: %w", err)
	}
	return paths, nil
}

func splitNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for index, value := range data {
		if value == 0 {
			return index + 1, data[:index], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func caseInsensitiveCollisions(paths []string) [][]string {
	byFoldedPath := make(map[string]map[string]struct{}, len(paths))
	for _, path := range paths {
		parts := strings.Split(path, "/")
		for count := 1; count <= len(parts); count++ {
			prefix := strings.Join(parts[:count], "/")
			key := strings.ToLower(prefix)
			if byFoldedPath[key] == nil {
				byFoldedPath[key] = map[string]struct{}{}
			}
			byFoldedPath[key][prefix] = struct{}{}
		}
	}
	collisions := make([][]string, 0)
	for _, candidateSet := range byFoldedPath {
		if len(candidateSet) < 2 {
			continue
		}
		candidates := make([]string, 0, len(candidateSet))
		for candidate := range candidateSet {
			candidates = append(candidates, candidate)
		}
		sort.Strings(candidates)
		collisions = append(collisions, candidates)
	}
	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i][0] < collisions[j][0]
	})
	return collisions
}
