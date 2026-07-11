package main

// loop-sum: tight integer loop 1..N accumulating (i & 1023) — exercises the
// raw loop / integer ALU. Kept < 2^53 so the result is exact in every language.

import (
	"fmt"
	"os"
)

const expected int64 = 334946

func main() {
	var acc int64 = 0
	var n int64 = 1000000000
	for i := int64(1); i <= n; i++ {
		acc += i & 1023
	}
	checksum := acc % 1000003
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
