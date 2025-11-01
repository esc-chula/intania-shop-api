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

setup:
	@echo "Installing development tools..."
	rustup component add rustfmt clippy
	cargo install cargo-llvm-cov
	cargo install cargo-audit
	@if ! command -v pre-commit &> /dev/null; then \
		echo "Installing pre-commit..."; \
		if command -v pip3 &> /dev/null; then \
			pip3 install pre-commit; \
		elif command -v pip &> /dev/null; then \
			pip install pre-commit; \
		elif command -v brew &> /dev/null; then \
			brew install pre-commit; \
		else \
			echo "Please install pre-commit manually: pip install pre-commit"; \
			exit 1; \
		fi; \
	fi
	pre-commit install

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