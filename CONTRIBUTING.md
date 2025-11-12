# Contributing to Cooldown Proxy

Thank you for your interest in contributing to Cooldown Proxy! This guide will help you get started.

## Getting Started

### Prerequisites

- Go 1.21+ installed
- Git installed
- Basic understanding of HTTP and Go

### Development Setup

1. **Fork and Clone**
   ```bash
   git clone https://github.com/your-username/cooldownp.git
   cd cooldownp
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   ```

3. **Run Tests**
   ```bash
   go test ./...
   ```

4. **Build the Project**
   ```bash
   go build -o cooldown-proxy ./cmd/proxy
   ```

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Make Your Changes

- Follow the existing code style
- Add tests for new functionality
- Update documentation as needed

### 3. Run Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/ratelimit/...
```

### 4. Check Code Quality

```bash
# Format code
go fmt ./...

# Run go vet
go vet ./...

# Run golint (if installed)
golint ./...
```

### 5. Commit Your Changes

```bash
git add .
git commit -m "feat: add your feature description"
```

### 6. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub.

## Code Style Guidelines

### Go Conventions

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` to format code
- Keep lines under 120 characters
- Use meaningful variable and function names

### Project Structure

```
internal/
├── config/          # Configuration handling
├── ratelimit/       # Rate limiting logic
├── proxy/          # HTTP proxy implementation
└── router/         # Request routing

cmd/proxy/          # Main application
docs/               # Documentation
```

### Testing Guidelines

- Write tests before implementation (TDD)
- Aim for high test coverage
- Test both success and error cases
- Use table-driven tests for multiple scenarios

#### Example Test Structure

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected ExpectedType
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    validInput,
            expected: expectedOutput,
            wantErr:  false,
        },
        {
            name:     "invalid input",
            input:    invalidInput,
            expected: nil,
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(result, tt.expected) {
                t.Errorf("FunctionName() = %v, want %v", result, tt.expected)
            }
        })
    }
}
```

## Types of Contributions

### Bug Fixes

1. Create an issue describing the bug
2. Write a test that reproduces the bug
3. Fix the bug
4. Ensure all tests pass
5. Update documentation if needed

### New Features

1. Create an issue to discuss the feature
2. Get feedback from maintainers
3. Implement the feature with tests
4. Update documentation
5. Add configuration options if needed

### Documentation

- Fix typos and grammar
- Improve examples
- Add missing documentation
- Translate documentation (if applicable)

### Performance Improvements

- Benchmark existing performance
- Implement improvements
- Add benchmarks for new code
- Document performance changes

## Configuration Changes

When adding new configuration options:

1. Update `internal/config/types.go`
2. Add validation in `internal/config/config.go`
3. Update `config.yaml.example`
4. Write tests for the new options
5. Update documentation

Example:

```go
// internal/config/types.go
type Config struct {
    Server           ServerConfig      `yaml:"server"`
    RateLimits       []RateLimitRule   `yaml:"rate_limits"`
    DefaultRateLimit *RateLimitRule    `yaml:"default_rate_limit"`
    NewFeature       NewFeatureConfig  `yaml:"new_feature"`
}

type NewFeatureConfig struct {
    Enabled bool   `yaml:"enabled"`
    Setting string `yaml:"setting"`
}
```

## Rate Limiting Contributions

When modifying rate limiting logic:

1. Understand the leaky bucket algorithm
2. Test with various rate scenarios
3. Ensure thread safety
4. Add comprehensive tests
5. Consider edge cases (zero rates, negative values, etc.)

## Testing Requirements

### Before Submitting

- [ ] All tests pass (`go test ./...`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] No `go vet` warnings (`go vet ./...`)
- [ ] New functionality has tests
- [ ] Documentation is updated
- [ ] Example configuration is updated

### Test Categories

1. **Unit Tests**: Test individual functions
2. **Integration Tests**: Test component interactions
3. **End-to-End Tests**: Test complete workflows
4. **Benchmark Tests**: Performance testing
5. **Example Tests**: Verify documentation examples

## Pull Request Process

### PR Title Format

Use conventional commit format:

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `test:` for test changes
- `refactor:` for refactoring
- `perf:` for performance improvements

### PR Description

Include:

1. **Problem**: What problem does this solve?
2. **Solution**: How did you solve it?
3. **Testing**: How did you test it?
4. **Breaking Changes**: Are there any breaking changes?

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] All tests pass
- [ ] New tests added
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Example configuration updated
```

## Release Process

Maintainers follow this process for releases:

1. Update version in `go.mod`
2. Update CHANGELOG.md
3. Create git tag
4. Build release binaries
5. Create GitHub release

## Getting Help

- **Issues**: Use GitHub issues for bugs and feature requests
- **Discussions**: Use GitHub Discussions for questions
- **Documentation**: Check existing documentation first

## Community Guidelines

- Be respectful and inclusive
- Provide constructive feedback
- Help others learn and grow
- Follow the [Code of Conduct](CODE_OF_CONDUCT.md)

## Recognition

Contributors are recognized in:
- README.md contributors section
- Release notes
- GitHub contributor statistics

Thank you for contributing to Cooldown Proxy! 🚀