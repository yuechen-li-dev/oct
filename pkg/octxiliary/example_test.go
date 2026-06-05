package octxiliary_test

import (
	"os"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

func ExampleNewDispatcher() {
	dispatcher := octxiliary.NewDispatcher("Echo")
	dispatcher.HandleFunc("EchoString", func(req octxiliary.Request) octxiliary.Response {
		text, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Err(req.ID, err)
		}
		return octxiliary.OkString(req.ID, text)
	})
	dispatcher.HandleFunc("ByteLength", func(req octxiliary.Request) octxiliary.Response {
		data, err := octxiliary.ArgBytes(req, 0)
		if err != nil {
			return octxiliary.Err(req.ID, err)
		}
		return octxiliary.OkInt(req.ID, len(data))
	})

	_ = octxiliary.Main(os.Stdin, os.Stdout, dispatcher.HandleRequest)
}
