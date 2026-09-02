# Contributing to Zygote

First, thank you for your interest in contributing to `zygote`! This project relies on the open-source community to grow and thrive.

This document outlines our development process, how to get your environment set up, and the standards we expect from contributions.

## Developer Certificate of Origin (DCO)

We enforce the Developer Certificate of Origin (DCO) on all pull requests. This ensures that contributors have the right to submit the code they are contributing.

All commits must include a `Signed-off-by` line. You can do this easily by passing the `-s` or `--signoff` flag to `git commit`:

```bash
git commit -s -m "feat(vfs): add amazing feature"
```

## Local Development Setup

To build and test zygote locally, you will need:
- Go 1.24 or later
- `make`
- `pre-commit` (Install via `brew install pre-commit` or `pip install pre-commit`)
- `golangci-lint` (Install via `brew install golangci-lint`)

### 1. Clone and Install Hooks
Once you have cloned the repository, install the pre-commit hooks. These hooks automatically enforce formatting, trailing whitespaces, and Apache-2.0 license headers.

```bash
pre-commit install
```

### 2. Makefile Commands
We use a standardized `Makefile` to orchestrate builds and tests.
- `make build`: Compiles the `zyg` CLI binary to `bin/zyg`.
- `make test`: Runs all unit tests with the race detector and coverage.
- `make lint`: Runs `golangci-lint` to check for code quality and security issues.
- `make check`: Runs both lint and test.
- `make all`: Runs check and build.

Before opening a pull request, please ensure that `make all` passes locally without any errors!

## Coding Standards

- **Conventional Commits**: We use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) (e.g., `feat(module): description`, `fix: ...`, `docs: ...`, `ci: ...`).
- **Dependencies**: The `zygote` core module (`vfs`, `journal`, `trace`) operates on a strict dependency budget (< 15 direct modules). Any new dependency must be thoroughly justified in your Pull Request.
- **Testing**: All features and fixes must include comprehensive tests. If you are fixing a bug, please include a test that reproduces the bug before applying the fix.

## Issue Etiquette
- If you find a security vulnerability, please refer to [SECURITY.md](SECURITY.md) and do **not** open a public issue.
- For bug reports and feature requests, please use the provided GitHub Issue templates and fill them out completely.

Thank you for contributing!
