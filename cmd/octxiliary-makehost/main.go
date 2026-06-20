// Command octxiliary-makehost serves make-authorized host capability primitives.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

const authorityMessage = "Make host capabilities are only available under oct make"

func main() { os.Exit(octxiliary.Main(os.Stdin, os.Stdout, dispatch)) }

func dispatch(req octxiliary.Request) octxiliary.Response {
	if req.Family != "Make" {
		return octxiliary.ErrString(req.ID, fmt.Sprintf("unknown family %q", req.Family))
	}
	if os.Getenv("OCT_MAKE_AUTHORITY") != "1" {
		return octxiliary.ErrString(req.ID, authorityMessage)
	}
	v, err := handle(req)
	if err != nil {
		return octxiliary.Err(req.ID, err)
	}
	return octxiliary.OkValue(req.ID, v)
}

func handle(req octxiliary.Request) (octxiliary.Value, error) {
	switch req.Function {
	case "MakeExec":
		prog, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		args, err := octxiliary.ArgStrings(req, 1)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return runProcess("", prog, args)
	case "MakeExecIn":
		cwd, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		prog, err := octxiliary.ArgString(req, 1)
		if err != nil {
			return octxiliary.Value{}, err
		}
		args, err := octxiliary.ArgStrings(req, 2)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return runProcess(cwd, prog, args)
	case "MakeTool":
		name, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		p, err := exec.LookPath(name)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.StringValue(p), nil
	case "MakeExists", "MakeIsFile", "MakeIsDir":
		p, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		info, statErr := os.Stat(p)
		if os.IsNotExist(statErr) {
			return octxiliary.BoolValue(false), nil
		}
		if statErr != nil {
			return octxiliary.BoolValue(false), nil
		}
		if req.Function == "MakeExists" {
			return octxiliary.BoolValue(true), nil
		}
		if req.Function == "MakeIsFile" {
			return octxiliary.BoolValue(info.Mode().IsRegular()), nil
		}
		return octxiliary.BoolValue(info.IsDir()), nil
	case "MakeMkdirAll":
		p, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.IntValue(0), os.MkdirAll(p, 0o755)
	case "MakeRemove":
		p, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.IntValue(0), os.Remove(p)
	case "MakeCopy":
		src, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		dst, err := octxiliary.ArgString(req, 1)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.IntValue(0), copyFile(src, dst)
	case "MakeReadText":
		p, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.StringValue(string(b)), nil
	case "MakeWriteText":
		p, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		text, err := octxiliary.ArgString(req, 1)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.IntValue(0), os.WriteFile(p, []byte(text), 0o644)
	case "MakeGlob":
		pat, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		matches, err := filepath.Glob(pat)
		if err != nil {
			return octxiliary.Value{}, err
		}
		sort.Strings(matches)
		for i := range matches {
			matches[i] = filepath.ToSlash(matches[i])
		}
		return octxiliary.StringsValue(matches), nil
	case "MakeModifiedTime":
		p, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		info, err := os.Stat(p)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.IntValue(int(info.ModTime().UnixNano())), nil
	case "MakeHashFile":
		p, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Value{}, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return octxiliary.Value{}, err
		}
		sum := sha256.Sum256(b)
		return octxiliary.StringValue(hex.EncodeToString(sum[:])), nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unsupported function %q", req.Function)
	}
}

func runProcess(cwd, program string, args []string) (octxiliary.Value, error) {
	cmd := exec.Command(program, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return octxiliary.Value{}, runErr
		}
	}
	return processResult(exitCode, out.String(), errb.String()), nil
}

func processResult(code int, stdout, stderr string) octxiliary.Value {
	return octxiliary.RecordValue("ProcessResult", []octxiliary.FieldValue{{Name: "ExitCode", Value: octxiliary.IntValue(code)}, {Name: "Stdout", Value: octxiliary.StringValue(stdout)}, {Name: "Stderr", Value: octxiliary.StringValue(stderr)}})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
