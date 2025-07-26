# Gismo Development Commands

## Essential Commands

### Testing
- `make test` - Run all tests with race detection and coverage
- `go test -v ./...` - Run tests without make wrapper
- `go test -v ./linters/golang` - Run specific package tests
- `make bench` - Run benchmarks

### Code Quality
- `make fmt` - Format code with gofmt and gofmt -s
- `make lint` - Run golangci-lint on all packages
- `golangci-lint run ./...` - Direct linting command

### Building
- `make build` - Build all binaries (gismo, gismo-init, gismo-show, etc.)
- `make install` - Install binaries to GOPATH/bin
- `go build ./cmd/gismo` - Build specific command

### Development Workflow
1. `make fmt` - Format code
2. `make lint` - Check for linting issues  
3. `make test` - Run full test suite
4. `make build` - Build binaries

### Cleanup
- `make clean` - Remove built binaries and cache
- `go clean -cache` - Clear Go build cache

## Critical Success Criteria
- **ALL** tests must pass (`make test` returns exit code 0)
- **ALL** linting checks must pass (`make lint` returns clean)
- **ALL** formatting must be correct (`make fmt` should make no changes)