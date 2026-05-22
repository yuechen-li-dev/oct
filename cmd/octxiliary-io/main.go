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
		req, _ := octxiliary.ParseRequest(frame)
		resp := octxiliary.Response{ID: req.ID}
		if !((req.Family == "IO.File" || req.Family == "File") && (req.Function == "FileReadText" || req.Function == "ReadText")) {
			resp.Error = "unsupported Octxiliary function family/function: " + req.Family + "/" + req.Function
		} else if text, readErr := interpret.FileReadTextForSidecar(req.Path); readErr != nil {
			resp.Error = readErr.Error()
		} else {
			resp.OK = true
			resp.Text = text
		}
		if err := octxiliary.WriteFrame(os.Stdout, octxiliary.EncodeResponse(resp)); err != nil {
			log.Fatal(err)
		}
	}
}
