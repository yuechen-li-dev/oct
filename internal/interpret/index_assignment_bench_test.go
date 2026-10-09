package interpret

import (
	"fmt"
	"testing"
)

// Measures the implementation cost independently of the language copy
// contracts under Language/Types/Arrays/valid.
func BenchmarkOwnedArrayIndexedWrite(b *testing.B) {
	for _, size := range []int{1024, 65536} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			target := Value{Kind: ValueArray, Array: make([]Value, size)}
			index := []int64{0}
			value := Value{Kind: ValueInt, Int: 1}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				target, err = assignNestedArrayIndex(target, index, value)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
