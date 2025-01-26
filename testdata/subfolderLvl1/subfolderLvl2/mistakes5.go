package main

import (
	"context" // duplicated import
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
)

type config struct {
	Port int
	Host string
}

type User struct {
	ID   int
	Name string
}

// Exported type with unexported field
type Client struct {
	apiKey string
	userID int
}

var unusedChan = make(chan int) // unused channel

func main() {
	// Unnecessary else clause
	if _, err := os.Open("file.txt"); err != nil {
		fmt.Println("Error opening file")
	} else {
		fmt.Println("File opened")
		return // redundant return
	}

	// Improper context usage
	ctx := context.Background()
	go fetchData(ctx)

	// Unused return value
	parseConfig("config.json")

	// Unhandled errors in defer
	f, _ := os.Create("temp.txt")
	defer f.Close()

	// Unkeyed struct literal
	c := config{8080, "localhost"}

	// Empty slice declaration
	var data []int = make([]int, 0)

	// Unused method receiver
	u := User{1, "Alice"}
	u.UnusedMethod()

	// Unnecessary blank identifier
	for _ = range []int{1, 2, 3} {
	}

	// Potential race condition
	counter := 0
	go func() {
		counter++
	}()
}

func fetchData(ctx context.Context) {
	// Ignoring context cancellation
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	// Using deprecated ioutil
	body, _ := ioutil.ReadAll(resp.Body)
	var result interface{}
	json.Unmarshal(body, &result) // Unhandled error
}

func parseConfig(path string) error {
	// Unchecked file existence
	data, _ := os.ReadFile(path)
	var cfg config
	return json.Unmarshal(data, &cfg) // Unhandled error
}

// Unused method receiver
func (u User) UnusedMethod() {
	// Empty branch
	if rand.Intn(10) > 5 {
	} else {
		fmt.Println("Random number")
	}
}

// Shadowing built-in package
var json struct{}

// Unused parameters
func handler(w http.ResponseWriter, r *http.Request) {
	// Using fmt instead of proper response
	fmt.Println("Received request") // Wrong output for HTTP handler
}

// Deprecated random seed
func init() {
	rand.Seed(42) // deprecated method
}

// Unreachable code
func unreachable() int {
	return 5
	fmt.Println("This will never execute") // unreachable
}
