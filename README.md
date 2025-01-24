# DeepRefactor

go run main.go golangci-lint run ./...


go run main.go golangci-lint run testdata/mistakes.go

curl http://localhost:11434/api/generate -d '{ "model": "deepseek-coder-v2", "prompt": "How are you today?", "stream": "false"}'