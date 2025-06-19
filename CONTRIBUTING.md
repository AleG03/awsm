# Contributing to AWSM

Thank you for considering contributing to AWSM! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and considerate of others when contributing to this project.

## How to Contribute

1. Fork the repository
2. Create a new branch for your feature or bugfix
3. Make your changes
4. Write tests for your changes
5. Run the existing tests to ensure nothing is broken
6. Submit a pull request

## Development Setup

1. Clone the repository
2. Install Go (version 1.18 or later)
3. Run `go mod download` to install dependencies
4. Run `go build` to build the project

## Testing

Run tests with:

```bash
go test ./...
```

## Code Style

- Follow standard Go code style and conventions
- Use `gofmt` to format your code
- Add comments for exported functions and types
- Keep functions small and focused

## Pull Request Process

1. Update the README.md with details of changes if appropriate
2. Update the tests to cover your changes
3. The PR should work on the main branch
4. Include a clear description of the changes

## License

By contributing to this project, you agree that your contributions will be licensed under the project's [Mozilla Public License 2.0](LICENSE).