# Concurrent Link Validator

A simple concurrent CLI tool written in Go that extracts and validates HTTP/HTTPS URLs from all `.md` files in a given directory.

## Features
- Standard library only.
- Worker pool pattern for concurrency.
- Configurable worker limit and target directory.

## Usage
```
go run main.go -dir /path/to/markdown/files -workers 10
```
