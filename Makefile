.PHONY: help build test clean lint fmt clippy ci setup pre-commit

help:
	@echo "Available commands:"
	@echo "  setup       - Install development dependencies and tools"
	@echo "  build       - Build the project"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format code with rustfmt"
	@echo "  clippy      - Run clippy lints"
	@echo "  lint        - Run fmt check and clippy"
	@echo "  ci          - Run full CI pipeline locally"
	@echo "  pre-commit  - Install pre-commit hooks"
	@echo "  clean       - Clean build artifacts"

# Detect OS
UNAME_S := $(shell uname -s)
ifeq ($(OS),Windows_NT)
    DETECTED_OS := Windows
else
    DETECTED_OS := $(UNAME_S)
endif

setup:
	@echo "Installing development tools for $(DETECTED_OS)..."
	@rustup component add rustfmt clippy
	@cargo install cargo-llvm-cov
	@cargo install cargo-audit
	@if ! command -v pre-commit >nul 2>&1 && ! command -v pre-commit >/dev/null 2>&1; then \
		echo "Installing pre-commit..."; \
		if command -v brew >nul 2>&1 || command -v brew >/dev/null 2>&1; then \
			brew install pre-commit; \
		elif command -v python3 >nul 2>&1 || command -v python3 >/dev/null 2>&1; then \
			if command -v pip3 >nul 2>&1 || command -v pip3 >/dev/null 2>&1; then \
				pip3 install pre-commit --user; \
			else \
				python3 -m pip install pre-commit --user; \
			fi; \
		elif command -v python >nul 2>&1 || command -v python >/dev/null 2>&1; then \
			if command -v pip >nul 2>&1 || command -v pip >/dev/null 2>&1; then \
				pip install pre-commit --user; \
			else \
				python -m pip install pre-commit --user; \
			fi; \
		elif command -v choco >nul 2>&1 || command -v choco >/dev/null 2>&1; then \
			choco install pre-commit; \
		else \
			echo "Please install pre-commit manually:"; \
			echo "  On macOS: brew install pre-commit"; \
			echo "  On Windows: choco install pre-commit"; \
			echo "  Or with pip: pip install pre-commit"; \
			echo "  Or visit: https://pre-commit.com/#installation"; \
			exit 1; \
		fi; \
	fi
	@pre-commit install
	@echo "Setup complete! Pre-commit hooks are now installed."

build:
	cargo build

test:
	@echo "Tests not implemented yet. Run 'cargo test' when ready."
	@echo "Use 'make build' to verify the project compiles."

test-build:
	cargo check --verbose

fmt:
	cargo fmt --all

fmt-check:
	cargo fmt --all -- --check

clippy:
	cargo clippy --all-targets --all-features -- -D warnings

audit:
	cargo audit

lint: fmt-check clippy

ci: lint audit build
	@echo "All CI checks passed!"

pre-commit:
	pre-commit install

clean:
	cargo clean

dev: fmt build
	@echo "Development workflow completed!"
