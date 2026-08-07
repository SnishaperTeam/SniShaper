# Contributing Guide

Thank you for your interest in **SniShaper**! We welcome all forms of contribution, including code, documentation, rules, testing, and feedback.

This guide will help you get started quickly.

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md). We are committed to maintaining a friendly, inclusive, and respectful community.

## How to Contribute

You can contribute in the following ways:

- Report bugs
- Suggest new features
- Improve documentation or the Wiki
- Submit code (bug fixes, optimizations, new features)
- Improve rule configurations
- Help with testing and feedback

### Submitting Issues

Please use the existing Issue templates whenever possible:

- **Bug Report**: Describe the problem, environment details, reproduction steps, and attach relevant files
- **Feature Request**: Explain the requirement, use cases, and expected behavior

Before opening a new issue, please search existing issues to avoid duplicates.

### Submitting Pull Requests

1. Fork this repository
2. Create a new branch (use clear names such as `fix/xxx` or `feat/xxx`)
3. Make your changes and ensure the project builds successfully
4. Write clear commit messages
5. Push to your fork
6. Open a Pull Request against the `main` branch of the upstream repository

In the PR description, please include:

- What was changed
- Why the change is needed
- Related Issue (use `Fixes #number` or `Closes #number` if applicable)
- Testing notes

## Development Environment

This project is built with **Wails v3**.

### Recommended Environment

- Go 1.25+
- Node.js 24+
- npm 11+
- gVisor (required for TUN mode)

### Quick Build

```powershell
# Clone the repository
git clone https://github.com/SniShaper/SniShaper.git
cd SniShaper

# Install frontend dependencies
cd frontend
npm install

# Build frontend
npm run build
cd ..

# Full build (recommended)
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1
```

You can also control the build with command-line parameters, for example:

```powershell
# Build frontend and backend, install dependencies
.\build_windows.ps1 -Build all -Lang cn -InstallDeps

# Silent mode (suitable for CI)
.\build_windows.ps1 -Silent
```

For more parameter details, see the "Build & Development" section in [README.md](README.md).

Build outputs:

- Frontend assets: `frontend/dist`
- Executable: `build/bin/snishaper.exe`

## Code Style Guidelines

- Keep the coding style consistent with the existing codebase
- Perform basic testing before submitting (the application should start and core features should work)
- Avoid mixing unrelated formatting changes or large refactors into feature PRs
- For significant changes, consider opening an Issue first for discussion

## Documentation and Rules

- Technical details and usage guides are mainly maintained in the [GitHub Wiki](https://github.com/SniShaper/SniShaper/wiki)
- Improvements to rules can be submitted directly in the `rules` directory or via Issue/PR
- Documentation improvements are also welcome

## License

This project is licensed under the [MIT License](LICENSE). By contributing, you agree that your contributions will be licensed under the same terms.

## Contact and Feedback

If you have questions, you can:

- Leave a comment on the relevant Issue or PR
- Check existing [Issues](https://github.com/SniShaper/SniShaper/issues) and [Discussions](https://github.com/SniShaper/SniShaper/discussions) (if enabled)

Thank you again for contributing!
