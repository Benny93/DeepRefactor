package main

import (
	"fmt"
	"io/ioutil" // Unused import
	"strings"
)

func main() {
	var x int = 42 // Redundant variable initialization
	y := "Hello, World!"
	fmt.Println(y)

	fmt.Println(sum(5, 10))
	fmt.Println(readFile("nonexistent.txt"))
}

// Function with poor naming and documentation issues
func sum(a int, b int) int {
	result := a + b // Inefficient variable usage, can return directly
	return result
}

// Function with unhandled errors
func readFile(filename string) string {
	content, _ := ioutil.ReadFile(filename) // Ignoring error here is a bad practice
	return strings.ToUpper(string(content))
}
