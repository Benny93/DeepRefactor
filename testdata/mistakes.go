package main

import (
	"fmt"
)

func main() {
	// Error: unused variable 'x'
	var x int
	fmt.Println(x)

	// Warning: ineffectual assignment to `v`
	for i := 0; i < 10; i++ {
		v := i // Should use "i" instead of "v"
	}

	// Error: SA4006 - don't use underscores in numeric literals
	const _ = 1_000_000

	// Warning: ineffectual assignment to `y`
	var y string
	getGreeting() // Call a function that doesn't assign `y`
}

func getGreeting() string {
	return "Hello, World!"
}
