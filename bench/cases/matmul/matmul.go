package main

// matmul: naive NxN integer matrix multiply — a memory-bound triple-nested
// loop. Matrices are stored flat (row-major, i*n+j) so every language shares
// the same layout and access pattern. checksum = sum of all C[i][j] mod MOD.

import (
	"fmt"
	"os"
)

const n = 512
const mod int64 = 1000003
const expected int64 = 434781

func main() {
	a := make([]int32, n*n)
	b := make([]int32, n*n)
	c := make([]int32, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			a[i*n+j] = int32((i + j) % 100)
			b[i*n+j] = int32((i * j) % 100)
		}
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			var s int32 = 0
			for k := 0; k < n; k++ {
				s += a[i*n+k] * b[k*n+j]
			}
			c[i*n+j] = s
		}
	}

	var sum int64 = 0
	for idx := 0; idx < n*n; idx++ {
		sum = (sum + int64(c[idx])) % mod
	}
	checksum := sum
	if expected < 0 {
		fmt.Fprintf(os.Stderr, "checksum=%d\n", checksum)
		os.Exit(0)
	}
	if checksum != expected {
		fmt.Fprintf(os.Stderr, "MISMATCH got=%d want=%d\n", checksum, expected)
		os.Exit(1)
	}
	os.Exit(0)
}
