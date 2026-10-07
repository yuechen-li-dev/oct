package octjson

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// The names of the functions that write JSON, as a message gives them.
const (
	OperationSave          = "Json.Save"
	OperationText          = "Json.Text"
	OperationArtifactWrite = "Artifact.WriteJson"
)

// TextAs writes value as the type schema describes and answers with the
// text. operation names the function that asked, and destination the file
// the text is for, or "" when there is none; a failure names both.
//
// It fails only on a value JSON cannot hold, such as a NaN. That failure
// stops the program (Stops): it is not an Error a program handles.
func TextAs(operation string, destination string, value Data, schema *Schema) (string, error) {
	text, err := Encode(value, schema)
	if err != nil {
		return "", stop{err.Text(operation, destination)}
	}
	return string(text), nil
}

// SaveAs is Json.Save: it writes value, as the type schema describes, to the
// file at path. The value is written whole or the file is not touched: a
// value JSON cannot hold fails before the file is opened.
//
// A file that cannot be written is an Error a program handles. A value JSON
// cannot hold stops the program (Stops).
func SaveAs(path string, value Data, schema *Schema) error {
	text, err := TextAs(OperationSave, path, value, schema)
	if err != nil {
		return err
	}
	if refusal := writeFile(path, []byte(text)); refusal != nil {
		return errors.New(refusal.Text(OperationSave, path))
	}
	return nil
}

// stop is a failure that stops the program.
type stop struct {
	text string
}

func (s stop) Error() string { return s.text }

// Stops reports whether a failure of TextAs or SaveAs stops the program. The
// other failures of SaveAs are Errors the program is given.
func Stops(err error) bool {
	var stopped stop
	return errors.As(err, &stopped)
}

// writeFile writes a file, and says why it could not in words of its own.
func writeFile(path string, text []byte) *Error {
	err := os.WriteFile(path, text, 0o644)
	if err == nil {
		return nil
	}
	info, statErr := os.Stat(path)
	_, parentErr := os.Stat(filepath.Dir(path))
	return &Error{Message: unwritableFile(err, statErr == nil && info.IsDir(), errors.Is(parentErr, fs.ErrNotExist))}
}

// unwritableFile is why a file could not be written.
func unwritableFile(err error, isDirectory bool, directoryMissing bool) string {
	switch {
	case isDirectory:
		return "this is a directory, not a file"
	case directoryMissing:
		return "the directory does not exist"
	case errors.Is(err, fs.ErrPermission):
		return "the file cannot be written: permission denied"
	default:
		return "the file cannot be written"
	}
}
