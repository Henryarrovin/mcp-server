run:
	go run main.go

build:
	go build -o bin/mcp-server .

.PHONY: run build