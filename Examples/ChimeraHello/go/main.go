package main

/*
#cgo CFLAGS: -I../rust
#cgo LDFLAGS: -L../rust/target/release -lchimera_rust -ldl -lpthread -lm
#include "chimera_rust.h"
*/
import "C"

import "fmt"

func main() {
	goNumber := int32(7)
	rustNumber := int32(C.rust_hello_number())
	total := goNumber + rustNumber
	fmt.Printf("chimera hello: go=%d rust=%d total=%d\n", goNumber, rustNumber, total)
}
