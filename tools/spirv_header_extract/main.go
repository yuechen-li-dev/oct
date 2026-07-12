package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: spirv_header_extract <input.h> <output.spv>")
		os.Exit(2)
	}
	text, err := os.ReadFile(os.Args[1])
	if err != nil { panic(err) }
	re := regexp.MustCompile(`0x[0-9a-fA-F]+`)
	words := re.FindAll(text, -1)
	if len(words) == 0 { panic("no hexadecimal SPIR-V words found") }
	out := make([]byte, len(words)*4)
	for i, raw := range words {
		value, err := strconv.ParseUint(string(raw[2:]), 16, 32)
		if err != nil { panic(err) }
		binary.LittleEndian.PutUint32(out[i*4:], uint32(value))
	}
	if err := os.WriteFile(os.Args[2], out, 0o644); err != nil { panic(err) }
}
