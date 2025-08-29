# Contributing to Mini-RAG

Thank you for your interest in contributing to Mini-RAG! This document provides guidelines and information for contributors.

## 🤝 How to Contribute

### Reporting Bugs

Before creating bug reports, please check the existing issues to see if the problem has already been reported. When creating a bug report, please include:

- **Clear title and description**
- **Steps to reproduce the issue**
- **Expected vs actual behavior**
- **Environment details** (OS, Go version, Ollama version)
- **Relevant logs or error messages**

Use the bug report template when creating new issues.

### Suggesting Enhancements

Enhancement suggestions are welcome! Please:

- **Check existing feature requests** to avoid duplicates
- **Provide a clear use case** for the enhancement
- **Describe the proposed solution** in detail
- **Consider backward compatibility** implications

### Pull Requests

1. **Fork the repository** and create your feature branch from `main`
2. **Make your changes** following the coding standards below  
3. **Add tests** for new functionality
4. **Update documentation** if needed
5. **Ensure all tests pass** with `make test`
6. **Run linting tools** with `make lint`
7. **Create a clear PR description** explaining your changes

## 🏗️ Development Setup

### Prerequisites

- Go 1.21+ with CGO support
- Ollama with an embedding model
- Make (for build automation)

### Setup Steps

```bash
# Clone your fork
git clone https://github.com/your-username/lil-rag.git
cd lil-rag

# Install dependencies
make deps

# Run tests to ensure everything works
make test

# Build the project
make build
```

### Development Workflow

```bash
# Format code
make fmt

# Run linting
make lint  

# Run tests
make test

# Run tests with coverage
make coverage

# Build and validate examples
make examples

# Full development build
make dev
```

## 📝 Coding Standards

### Go Code Style

- **Follow standard Go conventions** (gofmt, golint)
- **Use meaningful variable and function names**
- **Add comments for public APIs** and complex logic
- **Handle errors appropriately** - don't ignore them
- **Use structured logging** where appropriate
- **Keep functions focused** and reasonably sized

### Project Structure

```
lil-rag/
├── cmd/                    # Main applications
│   ├── lil-rag/          # CLI application
│   └── lil-rag-server/   # HTTP server
├── pkg/                    # Public library code
│   ├── minirag/           # Core RAG functionality
│   └── config/            # Configuration management
├── internal/               # Private application code
│   └── handlers/          # HTTP handlers
├── examples/               # Example programs
├── .github/               # GitHub templates and workflows
└── docs/                  # Additional documentation
```

### Code Organization

- **Keep packages focused** - each package should have a single responsibility
- **Use internal/ for private code** that shouldn't be imported by others
- **Put reusable code in pkg/** for public consumption
- **Add examples** for complex functionality
- **Include tests** alongside the code they test

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to index document %s: %w", id, err)
}

// Good: Check for specific error types when needed
if errors.Is(err, ErrDocumentNotFound) {
    return nil
}
```

### Logging

```go
// Use structured logging when possible
fmt.Printf("Indexing document '%s' with %d chunks\n", id, len(chunks))

// For errors, provide context
return fmt.Errorf("failed to compress chunk %d: %w", i, err)
```

## 🧪 Testing Guidelines

### Unit Tests

- **Test public APIs** comprehensively
- **Test error conditions** and edge cases
- **Use table-driven tests** for multiple scenarios
- **Mock external dependencies** (Ollama API calls)
- **Keep tests focused** and independent

Example:
```go
func TestIndex(t *testing.T) {
    tests := []struct {
        name    string
        text    string
        id      string
        wantErr bool
    }{
        {"valid input", "test content", "doc1", false},
        {"empty text", "", "doc1", true},
        {"empty id", "test content", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

- **Test CLI commands** end-to-end
- **Test HTTP API endpoints**
- **Test with real Ollama instance** when possible
- **Clean up test data** after each test

## 📚 Documentation

### Code Documentation

- **Document all public functions** with clear descriptions
- **Include parameter descriptions** and return values
- **Provide usage examples** for complex APIs
- **Document any special behavior** or limitations

### User Documentation  

- **Update README.md** for new features
- **Add examples** for new functionality  
- **Update API documentation** for endpoint changes
- **Include troubleshooting** for common issues

## 🚀 Release Process

### Versioning

Mini-RAG follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backward-compatible functionality additions
- **PATCH** version for backward-compatible bug fixes

### Creating Releases

1. **Update version numbers** in relevant files
2. **Update CHANGELOG.md** with release notes
3. **Create git tag** with version number
4. **Create GitHub release** with binaries
5. **Update documentation** if needed

## 💡 Feature Development

### Before Starting

- **Discuss major features** in an issue first
- **Consider backward compatibility**
- **Think about performance implications**
- **Plan for testing and documentation**

### Implementation Guidelines

- **Start with tests** - define expected behavior first
- **Keep changes focused** - one feature per PR
- **Consider configuration** - make features configurable when appropriate
- **Handle errors gracefully** - provide helpful error messages
- **Update examples** if the feature affects user workflows

## ❓ Getting Help

- **GitHub Issues** - For bug reports and feature requests
- **Discussions** - For questions and general discussion
- **Code Review** - Maintainers will review all PRs

## 🎯 Areas for Contribution

We especially welcome contributions in these areas:

- **Performance optimizations**
- **Additional file format support**
- **Enhanced CLI features**
- **API improvements**
- **Documentation improvements**
- **Test coverage**
- **Docker support**
- **CI/CD improvements**

## 👥 Community

- Be respectful and inclusive
- Help newcomers get started
- Share knowledge and best practices
- Follow the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md)

Thank you for contributing to Mini-RAG! 🎉