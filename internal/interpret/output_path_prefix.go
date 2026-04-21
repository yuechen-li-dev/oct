package interpret

import (
	"path/filepath"
	"sync"
)

var outputPathPrefixState struct {
	mu     sync.Mutex
	prefix string
}

func SetOutputPathPrefix(prefix string) func() {
	outputPathPrefixState.mu.Lock()
	previous := outputPathPrefixState.prefix
	outputPathPrefixState.prefix = prefix
	outputPathPrefixState.mu.Unlock()
	return func() {
		outputPathPrefixState.mu.Lock()
		outputPathPrefixState.prefix = previous
		outputPathPrefixState.mu.Unlock()
	}
}

func CurrentOutputPathPrefix() string {
	outputPathPrefixState.mu.Lock()
	defer outputPathPrefixState.mu.Unlock()
	return outputPathPrefixState.prefix
}

func attributedOutputPath(path string) string {
	outputPathPrefixState.mu.Lock()
	prefix := outputPathPrefixState.prefix
	outputPathPrefixState.mu.Unlock()
	if prefix == "" {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	return filepath.Join(dir, prefix+"."+base)
}
