package octjson

import (
	"errors"
	"io/fs"
	"os"
)

// The names of the Json functions, as a message gives them.
const (
	OperationLoad  = "Json.Load"
	OperationParse = "Json.Parse"
)

// LoadAs is Json.Load: it reads the file at path as the type schema
// describes. The error is the whole message, the same in both lanes.
func LoadAs(path string, schema *Schema, admit Admit) (Data, error) {
	text, err := readFile(path)
	if err != nil {
		return Data{}, errors.New(err.Text(OperationLoad, path))
	}
	return readAs(OperationLoad, path, text, schema, admit)
}

// ParseAs is Json.Parse: it reads text as the type schema describes.
func ParseAs(text string, schema *Schema, admit Admit) (Data, error) {
	return readAs(OperationParse, "", []byte(text), schema, admit)
}

func readAs(operation string, source string, text []byte, schema *Schema, admit Admit) (Data, error) {
	doc, err := Parse(text)
	if err != nil {
		return Data{}, errors.New(err.Text(operation, source))
	}
	value, err := Decode(doc, schema, admit)
	if err != nil {
		return Data{}, errors.New(err.Text(operation, source))
	}
	return value, nil
}

// readFile reads a file, and says why it could not in words of its own: no
// message of the operating system or of Go reaches a program.
func readFile(path string) ([]byte, *Error) {
	text, err := os.ReadFile(path)
	if err == nil {
		return text, nil
	}
	info, statErr := os.Stat(path)
	return nil, &Error{Message: unreadable(err, statErr == nil && info.IsDir())}
}

// unreadable is why a file could not be read.
func unreadable(err error, isDirectory bool) string {
	switch {
	case isDirectory:
		return "this is a directory, not a file"
	case errors.Is(err, fs.ErrNotExist):
		return "the file does not exist"
	case errors.Is(err, fs.ErrPermission):
		return "the file cannot be read: permission denied"
	default:
		return "the file cannot be read"
	}
}
