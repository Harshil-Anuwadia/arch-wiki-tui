# Contributing to archwiki-tui

First off, thank you for considering contributing to archwiki-tui! It's people like you who make the Arch Wiki accessible and enjoyable for everyone.

## Code of Conduct

This project and everyone participating in it is governed by a standard professional code of conduct. By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

- Use a clear and descriptive title for the issue.
- Describe the exact steps which reproduce the problem.
- Explain which behavior you expected to see instead and why.
- Include your environment details (OS, terminal emulator, Go version).

### Suggesting Enhancements

- Use a clear and descriptive title.
- Provide a step-by-step description of the suggested enhancement.
- Explain why this enhancement would be useful to most users.

### Pull Requests

1. Fork the repository.
2. Create a new branch for your feature or bug fix.
3. Add tests for your changes.
4. Ensure the test suite passes (`make test`).
5. Format your code (`make fmt`).
6. Submit a pull request.

## Style Guide

- Follow standard Go idioms.
- Use `go fmt` for formatting.
- Ensure all exported functions and types have comments.

## Development Setup

1. Install Go 1.25 or later.
2. Clone your fork.
3. Run `make tidy` to sync module dependencies.
4. Run `make build` to build the binary.

Happy hacking!
