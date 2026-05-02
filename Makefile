run:
	go run main.go

build:
	go build -o bin/mcp-server .

cli:
	cd cli && go run .

.PHONY: run build cli