package interpret

import (
	"errors"
	"os"
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

func FileExistsForSidecar(path string) (bool, error) {
	return fileExists(path)
}
