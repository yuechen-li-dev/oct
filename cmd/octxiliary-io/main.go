package main

import (
	"errors"
	"io"
	"log"
	"os"

	"oct/internal/interpret"
	"oct/internal/octxiliary"
)

func main() {
	if err := octxiliary.ReadHandshake(os.Stdin); err != nil {
		log.Fatal(err)
	}
	if err := octxiliary.WriteHandshake(os.Stdout); err != nil {
		log.Fatal(err)
	}
	for {
		frame, err := octxiliary.ReadFrame(os.Stdin)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			log.Fatal(err)
		}
		req, parseErr := octxiliary.ParseRequest(frame)
		resp := octxiliary.Response{ID: req.ID}
		if parseErr != nil {
			resp.Error = "invalid Octxiliary request payload: " + parseErr.Error()
			if err := octxiliary.WriteFrame(os.Stdout, octxiliary.EncodeResponse(resp)); err != nil {
				log.Fatal(err)
			}
			continue
		}
		if !isFileFamily(req.Family) {
			resp.Error = "unsupported Octxiliary function family/function: " + req.Family + "/" + req.Function
		} else {
			switch req.Function {
			case "FileReadText", "ReadText":
				if text, readErr := interpret.FileReadTextForSidecar(req.Path); readErr != nil {
					resp.Error = readErr.Error()
				} else {
					resp.OK = true
					resp.Text = text
				}
			case "FileWriteText", "WriteText":
				if writeErr := interpret.FileWriteTextForSidecar(req.Path, req.Text); writeErr != nil {
					resp.Error = writeErr.Error()
				} else {
					resp.OK = true
				}
			case "FileReadLines", "ReadLines":
				if lines, readErr := interpret.ReadLinesForSidecar(req.Path); readErr != nil {
					resp.Error = readErr.Error()
				} else {
					resp.OK = true
					resp.Lines = lines
				}
			case "FileWriteLines", "WriteLines":
				if writeErr := interpret.WriteLinesForSidecar(req.Path, req.Lines); writeErr != nil {
					resp.Error = writeErr.Error()
				} else {
					resp.OK = true
				}
			case "FileExists", "Exists":
				if exists, existsErr := interpret.FileExistsForSidecar(req.Path); existsErr != nil {
					resp.Error = existsErr.Error()
				} else {
					resp.OK = true
					resp.Exists = exists
					resp.HasExists = true
				}
			case "FileDelete", "Delete":
				if removeErr := interpret.FileDeleteForSidecar(req.Path); removeErr != nil {
					resp.Error = removeErr.Error()
				} else {
					resp.OK = true
				}
			case "DirectoryMake":
				if mkdirErr := interpret.DirectoryMakeForSidecar(req.Path); mkdirErr != nil {
					resp.Error = mkdirErr.Error()
				} else {
					resp.OK = true
				}
			case "DirectoryMakeAll":
				if mkdirErr := interpret.DirectoryMakeAllForSidecar(req.Path); mkdirErr != nil {
					resp.Error = mkdirErr.Error()
				} else {
					resp.OK = true
				}
			default:
				resp.Error = "unsupported Octxiliary function family/function: " + req.Family + "/" + req.Function
			}
		}
		if err := octxiliary.WriteFrame(os.Stdout, octxiliary.EncodeResponse(resp)); err != nil {
			log.Fatal(err)
		}
	}
}

func isFileFamily(family string) bool {
	return family == "IO.File" || family == "File" || family == "Directory" || family == "IO.Directory"
}
