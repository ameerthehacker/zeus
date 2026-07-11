package main

// prime-sieve: Sieve of Eratosthenes up to LIMIT, counts primes.
// Exercises array allocation, indexing and nested loops.
// The `p <= LIMIT/p` guard keeps p*p within 32-bit range for every language.

import (
	"fmt"
	"os"
)

const limit = 30000000
const expected int64 = 1857859

func main() {
	sieve := make([]byte, limit+1)
	count := int64(0)
	for p := 2; p <= limit; p++ {
		if sieve[p] == 0 {
			count++
			if p <= limit/p {
				for j := p * p; j <= limit; j += p {
					sieve[j] = 1
				}
			}
		}
	}
	checksum := count
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
