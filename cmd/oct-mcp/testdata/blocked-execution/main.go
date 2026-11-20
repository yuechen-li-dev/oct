// This child blocks without spawning descendants, isolating the MCP process
// deadline contract from Oct compilation and language execution.
package main

import "time"

func main() { time.Sleep(time.Minute) }
