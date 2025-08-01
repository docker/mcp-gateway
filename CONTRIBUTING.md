# Contributing guide

## Building from Source

```bash
# Build the binary
make docker-mcp

# Cross-compile for all platforms
make docker-mcp-cross

# Run tests
make test
```

## Code Quality

```bash
# Format code
make format

# Run linter
make lint
```

## Open a Pull Request

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and linting (`make unit-tests lint`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Code of Conduct

This project follows a Code of Conduct. Please review it in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
