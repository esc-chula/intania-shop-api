#!/bin/bash

# Development Environment Setup Script for Intania Shop API
# This script installs pre-commit hooks and sets up the development environment

set -e

echo "Setting up development environment for Intania Shop API..."

# Check if pre-commit is installed
if ! command -v pre-commit &> /dev/null; then
    echo "Installing pre-commit..."

    # Try different installation methods
    if command -v pip3 &> /dev/null; then
        pip3 install pre-commit
    elif command -v pip &> /dev/null; then
        pip install pre-commit
    elif command -v brew &> /dev/null; then
        brew install pre-commit
    else
        echo "❌ Error: Could not find pip or brew. Please install pre-commit manually:"
        echo "   pip install pre-commit"
        echo "   or visit: https://pre-commit.com/#installation"
        exit 1
    fi
else
    echo "pre-commit is already installed"
fi

echo "Installing pre-commit hooks..."
pre-commit install

echo "Verifying pre-commit installation..."
pre-commit --version

echo ""
echo "Development environment setup complete!"
echo ""
echo "Pre-commit hooks are now active and will run on:"
echo "   - git commit"
echo "   - git push"
echo ""
echo "You can also run hooks manually:"
echo "   pre-commit run --all-files  # Run on all files"
echo "   make lint                   # Run formatting and clippy"
echo "   make test                   # Run all tests"
echo ""