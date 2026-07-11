package main

// sort: insertion sort (O(n^2)) of an LCG-filled array — exercises array
// get/set and comparisons. The LCG uses uint32 wraparound so the exact same
// sequence is reproducible in Go, Zeus (i64 + mask) and JS (Math.imul).
// checksum = positional weighted sum mod MOD (detects both content and order).

import (
	"fmt"
	"os"
)

const n = 20000
const mod int64 = 1000003
const expected int64 = 861863

func main() {
	arr := make([]int32, n)
	var x uint32 = 12345
	for i := 0; i < n; i++ {
		x = x*1103515245 + 12345
		arr[i] = int32((x >> 16) & 0xffff)
	}

	for i := 1; i < n; i++ {
		key := arr[i]
		j := i - 1
		for j >= 0 && arr[j] > key {
			arr[j+1] = arr[j]
			j--
		}
		arr[j+1] = key
	}

	var sum int64 = 0
	for i := 0; i < n; i++ {
		sum = (sum + int64(i+1)*int64(arr[i])) % mod
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
