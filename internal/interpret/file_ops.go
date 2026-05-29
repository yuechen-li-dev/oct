package interpret

import (
	"errors"
	"os"
	"sort"
	"strings"
)

func fileReadText(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", mapPathError(path, err)
	}
	return string(contents), nil
}

func fileWriteText(path string, text string) error {
	if writeErr := os.WriteFile(path, []byte(text), 0o644); writeErr != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			if retryErr := os.WriteFile(path, []byte(text), 0o644); retryErr == nil {
				return nil
			} else {
				return mapPathError(path, retryErr)
			}
		}
		return mapPathError(path, writeErr)
	}
	return nil
}

func readLines(path string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, mapPathError(path, err)
	}
	lines := splitLinesPreservingTerminal(strings.ReplaceAll(string(contents), "\r\n", "\n"))
	return lines, nil
}

func writeLines(path string, lines []string) error {
	payload := ""
	if len(lines) > 0 {
		payload = strings.Join(lines, "\n") + "\n"
	}
	if writeErr := os.WriteFile(path, []byte(payload), 0o644); writeErr != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			if retryErr := os.WriteFile(path, []byte(payload), 0o644); retryErr == nil {
				return nil
			} else {
				return mapPathError(path, retryErr)
			}
		}
		return mapPathError(path, writeErr)
	}
	return nil
}
func fileExists(path string) (bool, error) {
	_, statErr := os.Stat(path)
	if statErr == nil {
		return true, nil
	}
	if errors.Is(statErr, os.ErrNotExist) {
		return false, nil
	}
	return false, mapPathError(path, statErr)
}

func FileReadTextForSidecar(path string) (string, error) {
	return fileReadText(path)
}

func FileWriteTextForSidecar(path string, text string) error {
	return fileWriteText(path, text)
}

func FileReadBytesForSidecar(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, mapPathError(path, err)
	}
	return contents, nil
}

func FileWriteBytesForSidecar(path string, data []byte) error {
	if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			if retryErr := os.WriteFile(path, data, 0o644); retryErr == nil {
				return nil
			} else {
				return mapPathError(path, retryErr)
			}
		}
		return mapPathError(path, writeErr)
	}
	return nil
}

func FileExistsForSidecar(path string) (bool, error) {
	return fileExists(path)
}

func FileDeleteForSidecar(path string) error {
	if err := os.Remove(path); err != nil {
		return mapPathError(path, err)
	}
	return nil
}

func DirectoryMakeForSidecar(path string) error {
	if err := os.Mkdir(path, 0o755); err != nil {
		return mapPathError(path, err)
	}
	return nil
}

func DirectoryMakeAllForSidecar(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return mapPathError(path, err)
	}
	return nil
}

func DirectoryListForSidecar(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, mapPathError(path, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func DirectoryRemoveAllForSidecar(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return mapPathError(path, err)
	}
	return nil
}

func ReadLinesForSidecar(path string) ([]string, error) {
	return readLines(path)
}

func WriteLinesForSidecar(path string, lines []string) error {
	return writeLines(path, lines)
}
