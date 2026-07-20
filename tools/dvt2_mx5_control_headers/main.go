// Command dvt2_mx5_control_headers materializes diagnostic-only native headers
// from the controlled SPIR-V binaries. It is intentionally not the production
// generator and names the output as an explicitly selected control artifact.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		panic("usage: dvt2_mx5_control_headers <input.spv> <output.h> <symbol>")
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	if len(b) == 0 || len(b)%4 != 0 {
		panic("SPIR-V must be non-empty words")
	}
	guard := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_", "/", "_").Replace(filepath.Base(os.Args[2])))
	var out strings.Builder
	fmt.Fprintf(&out, "#ifndef OCT_DVT2_MX5_CONTROL_%s\n#define OCT_DVT2_MX5_CONTROL_%s\n\n#include <stdint.h>\n#include <stddef.h>\n\n", guard, guard)
	fmt.Fprintf(&out, "// Diagnostic-only controlled SPIR-V artifact; never production-selected.\nstatic const uint32_t %s[] = {\n", os.Args[3])
	for i := 0; i < len(b); i += 4 {
		if i%16 == 0 {
			out.WriteString("    ")
		}
		fmt.Fprintf(&out, "0x%02x%02x%02x%02xu,", b[i+3], b[i+2], b[i+1], b[i])
		if i%16 == 12 {
			out.WriteByte('\n')
		} else {
			out.WriteByte(' ')
		}
	}
	fmt.Fprintf(&out, "\n};\nstatic const size_t %s_size_bytes = sizeof(%s);\n\n#endif\n", os.Args[3], os.Args[3])
	if err := os.WriteFile(os.Args[2], []byte(out.String()), 0644); err != nil {
		panic(err)
	}
}
