package source

import (
	"fmt"
	"os"
)

type File struct {
	Path string
	Text string
}

func Load(path string) (File, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, fmt.Errorf("source file not found: %s", path)
		}
		return File{}, fmt.Errorf("load source %s: %w", path, err)
	}

	return File{
		Path: path,
		Text: string(text),
	}, nil
}
