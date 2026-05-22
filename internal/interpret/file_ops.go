package interpret

import "os"

func fileReadText(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", mapPathError(path, err)
	}
	return string(contents), nil
}

func FileReadTextForSidecar(path string) (string, error) {
	return fileReadText(path)
}
