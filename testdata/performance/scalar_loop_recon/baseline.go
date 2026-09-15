package main

import "fmt"

func main() {
	total := 0.0
	for index := 0; index < 20_000_000; index++ {
		total = total + float64(index)*0.0000001
	}
	fmt.Println(total)
}
