package octjson

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestTextAs(t *testing.T) {
	config := record("Config", "Name", stringType, "Levels", arrayOf(floatType))
	text, err := TextAs(OperationText, "", rec("Config", "Name", s("api"), "Levels", list(f(1), f(2.5))), config)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"Name\": \"api\",\n  \"Levels\": [1.0, 2.5]\n}\n"; text != want {
		t.Errorf("got %q, want %q", text, want)
	}

	// A value JSON cannot hold stops the program, and the failure names the
	// function, the file when there is one, and the place in the value.
	unwritable := rec("Config", "Name", s("api"), "Levels", list(f(1), f(math.NaN())))
	cases := []struct {
		operation, destination, want string
	}{
		{OperationText, "", "Json.Text: $.Levels[1]: NaN has no JSON form"},
		{OperationSave, "out.json", "Json.Save: out.json: $.Levels[1]: NaN has no JSON form"},
		{OperationArtifactWrite, "report.json", "Artifact.WriteJson: report.json: $.Levels[1]: NaN has no JSON form"},
	}
	for _, c := range cases {
		_, err := TextAs(c.operation, c.destination, unwritable, config)
		if err == nil || err.Error() != c.want {
			t.Errorf("got %v, want %s", err, c.want)
		}
		if !Stops(err) {
			t.Errorf("%v does not stop the program", err)
		}
	}

	// A value that is not what its schema says is a defect of the lane, and
	// stops the program too.
	_, err = TextAs(OperationText, "", s("api"), config)
	if err == nil || !Stops(err) {
		t.Errorf("a value of the wrong shape: %v", err)
	}
}

func TestSaveAs(t *testing.T) {
	directory := t.TempDir()
	levels := arrayOf(floatType)
	path := filepath.Join(directory, "levels.json")

	if err := SaveAs(path, list(f(1), f(2.5)), levels); err != nil {
		t.Fatal(err)
	}
	written, readErr := os.ReadFile(path)
	if readErr != nil || string(written) != "[1.0, 2.5]\n" {
		t.Errorf("wrote %q, %v", written, readErr)
	}
	back, err := LoadAs(path, levels, nil)
	if err != nil || len(back.Array) != 2 || back.Array[1].Float != 2.5 {
		t.Errorf("read back %+v, %v", back, err)
	}

	// Saving again replaces the file.
	if err := SaveAs(path, list(), levels); err != nil {
		t.Fatal(err)
	}
	if written, _ := os.ReadFile(path); string(written) != "[]\n" {
		t.Errorf("wrote %q over the first file", written)
	}

	// A value JSON cannot hold stops the program before the file is touched.
	err = SaveAs(path, list(f(math.Inf(1))), levels)
	if want := "Json.Save: " + path + ": $[0]: an infinity has no JSON form"; err == nil || err.Error() != want || !Stops(err) {
		t.Errorf("got %v (stops: %v), want %s", err, Stops(err), want)
	}
	if written, _ := os.ReadFile(path); string(written) != "[]\n" {
		t.Errorf("the file was touched: %q", written)
	}

	// A file that cannot be written is an Error the program is given.
	missing := filepath.Join(directory, "no_such_directory", "levels.json")
	for target, want := range map[string]string{
		missing:   "Json.Save: " + missing + ": the directory does not exist",
		directory: "Json.Save: " + directory + ": this is a directory, not a file",
	} {
		err := SaveAs(target, list(), levels)
		if err == nil || err.Error() != want {
			t.Errorf("got %v, want %s", err, want)
		}
		if Stops(err) {
			t.Errorf("%v stops the program; it is an Error", err)
		}
	}
}

func TestStops(t *testing.T) {
	if Stops(nil) || Stops(errors.New("an Error")) {
		t.Errorf("an ordinary failure, or none, stops the program")
	}
}

// Why a file cannot be written is said in words of this package's own.
func TestUnwritableFile(t *testing.T) {
	cases := []struct {
		err              error
		isDirectory      bool
		directoryMissing bool
		want             string
	}{
		{&fs.PathError{Op: "open", Path: "a", Err: errors.New("is a directory")}, true, false, "this is a directory, not a file"},
		{&fs.PathError{Op: "open", Path: "d/a.json", Err: fs.ErrNotExist}, false, true, "the directory does not exist"},
		{&fs.PathError{Op: "open", Path: "a.json", Err: fs.ErrPermission}, false, false, "the file cannot be written: permission denied"},
		{errors.New("no space left on device"), false, false, "the file cannot be written"},
	}
	for _, c := range cases {
		if got := unwritableFile(c.err, c.isDirectory, c.directoryMissing); got != c.want {
			t.Errorf("unwritableFile(%v, %v, %v) = %q, want %q", c.err, c.isDirectory, c.directoryMissing, got, c.want)
		}
	}
}
