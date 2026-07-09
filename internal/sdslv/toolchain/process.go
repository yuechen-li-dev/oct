package toolchain

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Command struct {
	Program string
	Args    []string
	Dir     string
	Env     []string
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type CommandRunner func(Command) (CommandResult, error)

func defaultCommandRunner(cmd Command) (CommandResult, error) {
	command := exec.Command(cmd.Program, cmd.Args...)
	if cmd.Dir != "" {
		command.Dir = cmd.Dir
	}
	if len(cmd.Env) > 0 {
		command.Env = mergeEnv(os.Environ(), cmd.Env)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exitCode := 0
	if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
	}
	return CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}

func mergeEnv(base, overlay []string) []string {
	index := map[string]int{}
	out := append([]string(nil), base...)
	for i, kv := range out {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			index[kv[:eq]] = i
		}
	}
	for _, kv := range overlay {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			key := kv[:eq]
			if i, ok := index[key]; ok {
				out[i] = kv
			} else {
				index[key] = len(out)
				out = append(out, kv)
			}
		}
	}
	return out
}

func resolveCommandDir(root, dir string) string {
	if dir == "" {
		return root
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(root, dir)
}
