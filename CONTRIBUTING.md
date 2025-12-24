# Contributing to SMSpit

First off, thank you for considering contributing to SMSpit! 🎉

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues as you might find that you don't need to create one. When you do create a bug report, include as many details as possible:

- A clear and descriptive title
- Exact steps to reproduce
- What you expected vs. what actually happened
- Screenshots if applicable
- Your environment (OS, Docker version, etc.)

### Suggesting Features

Feature suggestions are tracked as GitHub issues. When creating a feature request, include:

- A clear and descriptive title
- Detailed description of the proposed feature
- Why this feature would be useful
- Any examples from similar tools (if applicable)

### Pull Requests

1. Fork the repo and create your branch from `main`
2. If you've added code, add tests
3. If you've changed APIs, update the documentation
4. Ensure the test suite passes
5. Make sure your code lints
6. Issue the PR!

## Development Setup

### Prerequisites

- Go 1.21+
- Docker (for testing)
- Node.js 18+ (for UI development, optional)

### Building from Source

```bash
# Clone the repo
git clone https://github.com/substrate-app/smspit.git
cd smspit

# Build
go build -o smspit ./src/

# Run
./smspit
```

### Running Tests

```bash
go test ./...
```

### Building Docker Image

```bash
docker build -t smspit:dev -f src/Dockerfile src/
docker run -p 8080:8080 -p 9080:9080 smspit:dev
```

## Code Style

- Follow standard Go formatting (`gofmt`)
- Write meaningful commit messages
- Keep functions focused and small
- Add comments for complex logic

## Project Structure

```
src/
├── main.go          # Main server code
├── static/          # Web UI files
│   └── index.html   # Single-page application
├── go.mod           # Go module definition
└── Dockerfile       # Container build

module.yaml          # Substrate module definition
README.md            # User documentation
CONTRIBUTING.md      # This file
```

## UI Development

The web UI is a single HTML file with embedded CSS/JS for simplicity. This makes it easy to embed in the Go binary and keeps the deployment simple.

If you want to make UI changes:

1. Edit `src/static/index.html`
2. Rebuild the Go binary
3. Test in browser

## Release Process

Releases are handled by the Substrate team. Version numbers follow [Semantic Versioning](https://semver.org/).

## Questions?

Feel free to open an issue with your question or reach out to the Substrate community.

---

**Thank you for contributing to SMSpit!** 📱

