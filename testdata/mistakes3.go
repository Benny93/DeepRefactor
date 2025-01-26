package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	logFile, _ := os.Create("log.txt") // Error not handled here
	defer logFile.Close()              // Deferring close without ensuring logFile was created successfully

	writeLog(logFile, "Application started")
	simulateWork()
	writeLog(logFile, "Application finished")
}

// Function writes logs to a file without handling potential errors from WriteString
func writeLog(file *os.File, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	file.WriteString(fmt.Sprintf("%s - %s\n", timestamp, message)) // Ignoring error returned by WriteString
}

// Function with an ignored error-prone operation
func simulateWork() {
	fmt.Println("Simulating work...")
	_, err := os.Open("nonexistent_file.txt") // Error ignored completely
	if err != nil {
		fmt.Println("An error occurred, but I am ignoring it!") // Inefficient error handling
	}
}
