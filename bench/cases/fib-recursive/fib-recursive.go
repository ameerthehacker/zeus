package main

// fib-recursive: naive recursive Fibonacci — exercises function-call overhead.
// checksum = fib(40) = 102334155

import (
	"fmt"
	"os"
)

const expected int64 = 102334155

func fib(n int32) int32 {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	checksum := int64(fib(40))
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
