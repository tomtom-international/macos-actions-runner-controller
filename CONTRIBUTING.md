# Contributing to macOS GitHub Actions Runner Controller

Thank you for your interest in contributing to the macOS GitHub Actions Runner Controller! This document provides a set of guidelines for contributing to the project.

## Code of Conduct

Please be respectful and considerate when interacting with other contributors. We're all here to build something great together.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/tomtom-international/macos-actions-runner-controller.git
   cd macos-actions-runner-controller
   ```
3. Create a new branch for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Environment

1. Install Go (1.18 or later recommended)
2. For Tarter development, you'll need access to a macOS environment
3. Install dependencies:
   ```bash
   go mod download
   ```

## Making Changes

1. Make your changes in your feature branch
2. Add or update tests as necessary
3. Ensure your code follows the project's code style
4. Run the tests to make sure everything works:
   ```bash
   go test ./...
   ```
5. Make sure your code passes linting:
   ```bash
   golangci-lint run
   ```

## Submitting Changes

1. Commit your changes:
   ```bash
   git commit -m "feat: Add your feature description"
   ```
2. Push to your fork:
   ```bash
   git push origin feat/your-feature-name
   ```
3. Submit a pull request to the main repository
4. In your pull request description, explain your changes and the problem they solve

## Pull Request Process

1. Update the README.md with details of changes if appropriate
2. Ensure your PR passes all CI checks
3. A maintainer will review your PR and might request changes
4. Once approved, your PR will be merged

## Coding Standards

- Follow standard Go coding conventions
- Use meaningful variable and function names
- Write comments to explain complex logic
- Break large functions into smaller, more manageable pieces
- Write unit tests for your code

## Reporting Bugs

If you find a bug, please create an issue in the GitHub repository with:

1. A clear, descriptive title
2. A detailed description of the issue
3. Steps to reproduce the problem
4. Expected behavior
5. Actual behavior
6. Any relevant logs or error messages
7. Your environment (OS, Go version, etc.)

## Feature Requests

Feature requests are welcome! Please submit them as issues and:

1. Use a clear, descriptive title
2. Describe the feature you'd like to see
3. Explain why this feature would be useful to the project
4. Suggest a possible implementation approach if you have one in mind

## Questions?

If you have any questions about contributing, please open an issue and we'll be happy to help!
Thank you for contributing to the macOS GitHub Actions Runner Controller!