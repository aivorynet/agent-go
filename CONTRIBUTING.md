# Contributing to AIVory Monitor Go Agent

Thank you for your interest in contributing to the AIVory Monitor Go Agent. Contributions of all kinds are welcome -- bug reports, feature requests, documentation improvements, and code changes.

## How to Contribute

- **Bug reports**: Open an issue at [GitHub Issues](https://github.com/aivorynet/agent-go/issues) with a clear description, steps to reproduce, and your environment details (Go version, OS, architecture).
- **Feature requests**: Open an issue describing the use case and proposed behavior.
- **Pull requests**: See the Pull Request Process below.

## Development Setup

### Prerequisites

- Go 1.21 or later

### Build and Test

```bash
cd monitor-agents/agent-go
go build ./...
go test ./...
```

### Running the Agent

Import the agent package in your application and call the initialization function at startup. See the README for integration details.

## Coding Standards

- Follow the existing code style in the repository.
- Write tests for all new features and bug fixes.
- Run `go vet ./...` and resolve all warnings before submitting.
- Use `gofmt` or `goimports` to format code.
- Follow the [Effective Go](https://go.dev/doc/effective_go) guidelines.
- Keep eBPF and runtime hook usage well-documented and gated behind build tags where appropriate.

## Pull Request Process

1. Fork the repository and create a feature branch from `main`.
2. Make your changes and write tests.
3. Ensure all tests pass (`go test ./...`) and vet is clean (`go vet ./...`).
4. Submit a pull request on [GitHub](https://github.com/aivorynet/agent-go) or GitLab.
5. All pull requests require at least one review before merge.

## Reporting Bugs

Use [GitHub Issues](https://github.com/aivorynet/agent-go/issues). Include:

- Go version (`go version`) and OS
- Agent version
- Panic output or error logs
- Minimal reproduction steps

## Security

Do not open public issues for security vulnerabilities. Report them to **security@aivory.net**. See [SECURITY.md](SECURITY.md) for details.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
