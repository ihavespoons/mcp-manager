# Task Completion Checklist

When completing a development task, follow this checklist:

## 1. Code Quality
- [ ] **Format code**: Run `make fmt`
  - Ensures all Go code follows standard formatting
- [ ] **Run go vet**: Run `make vet`
  - Checks for suspicious code constructs
- [ ] **Run linter**: Run `make lint` (if golangci-lint is installed)
  - Performs comprehensive linting checks

## 2. Testing
- [ ] **Run tests**: Run `make test`
  - All tests should pass
  - Includes race detection
- [ ] **Check coverage**: Review `coverage.out` if needed
  - Ensure new code has adequate test coverage

## 3. Build Verification
- [ ] **Build successfully**: Run `make build`
  - Verify the binary builds without errors
- [ ] **Test binary**: Run `./mcp-manager --help` or relevant commands
  - Ensure the binary works as expected

## 4. Module Management
- [ ] **Tidy dependencies**: Run `make tidy`
  - Clean up go.mod and go.sum
  - Only if dependencies were added or removed

## 5. Documentation
- [ ] Update relevant documentation if needed
  - Add/update comments for exported functions and types
  - Update README.md if user-facing features changed

## Typical Workflow After Changes
```bash
# 1. Format and check code
make fmt
make vet

# 2. Run tests
make test

# 3. Build
make build

# 4. Tidy (if dependencies changed)
make tidy

# 5. Test the binary
./mcp-manager --help
```

## Notes
- The project is on Darwin (macOS), so all commands should work on macOS
- Docker must be running if testing Docker-related functionality
- `golangci-lint` may need to be installed separately for `make lint` to work
