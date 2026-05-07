run:
	go run main.go

build:
	go build -o bin/mcp-server .

cli:
	cd cli && go run .

cli-build:
	cd cli && go build -o ../bin/mcp-cli .

.PHONY: run build cli cli-build