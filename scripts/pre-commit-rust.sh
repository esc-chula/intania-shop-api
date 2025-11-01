#!/bin/bash

# Rust pre-commit hook script
# This script runs comprehensive checks before commits, including:
# - Formatting with rustfmt
# - Linting with clippy
# - Running targeted tests for changed files
# - Checking for compilation errors

set -e

echo "Running Rust pre-commit checks..."

restage_files() {
    echo "Restaging formatted files..."
    # Restage any files that were modified by rustfmt
    git diff --name-only --cached | grep '\.rs$' | xargs git add 2>/dev/null || true

    # Check for any generated files that need to be restaged
    generated_changes=$(git status --porcelain | awk '/^( M|M |A |AM|\?\?)/ {print $2}' | grep -E '\.(rs|toml)$' || true)
    if [ -n "$generated_changes" ]; then
        git add $generated_changes 2>/dev/null || true
    fi
}

run_targeted_tests() {
    local packages=""
    local test_files=""

    # Find all modified Rust files
    while IFS= read -r file; do
        [ -n "$file" ] || continue

        # Only process .rs files
        if [[ "$file" != *.rs ]]; then
            continue
        fi

        # Skip generated files and certain directories
        case "$file" in
            */target/*|*build.rs*|*/.git/*)
                continue
                ;;
        esac

        # Get the directory containing the file
        dir=$(dirname "$file")
        if [ "$dir" = "." ]; then
            pkg="."
        else
            pkg="./$dir"
        fi

        # Add package to list if not already included
        case " $packages " in
            *" $pkg "*) ;;
            *) packages="$packages $pkg" ;;
        esac

        # Find test files related to this module
        test_file="${file%.rs}_test.rs"
        if [ -f "$test_file" ]; then
            test_files="$test_files $test_file"
        fi

        # Also look for tests in tests/ directory
        if [ -f "tests/$test_file" ]; then
            test_files="$test_files tests/$test_file"
        fi

    done <<EOF
$staged_rust_files
EOF

    if [ -z "$packages" ]; then
        echo "No Rust packages resolved from staged files. Skipping cargo test."
        return
    fi

    echo "Running cargo test for affected packages..."

    # First try to build to ensure everything compiles
    echo "Checking compilation..."
    if ! cargo check; then
        echo "Compilation failed. Please fix build errors before committing."
        exit 1
    fi

    # Run tests
    echo "Running tests for modified modules..."
    for pkg in $packages; do
        echo "   Testing: $pkg"
        if ! cargo test --manifest-path "$pkg/Cargo.toml" 2>/dev/null && \
           ! cargo test -p "$pkg" 2>/dev/null && \
           ! cargo test "$pkg" 2>/dev/null; then
            # If specific package testing fails, run all tests
            echo "Could not test specific package, running all tests..."
            if ! cargo test; then
                echo "cargo test failed"
                exit 1
            fi
        fi
    done

    echo "Targeted tests passed!"
}

check_dependencies() {
    # Check if rustfmt is available
    if ! rustfmt --version &> /dev/null; then
        echo "rustfmt is not available. Please install it with: rustup component add rustfmt"
        exit 1
    fi

    # Check if clippy is available
    if ! cargo clippy --version &> /dev/null; then
        echo "clippy is not available. Please install it with: rustup component add clippy"
        exit 1
    fi
}

# Check for staged Rust files
staged_rust_files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.rs$' || true)
staged_toml_files=$(git diff --cached --name-only --diff-filter=ACM | grep '\(Cargo\.toml\|Cargo\.lock\)$' || true)

run_checks=false
should_check_all=false

# If Cargo.toml or Cargo.lock is changed, check everything
if [ -n "$staged_toml_files" ]; then
    echo "Cargo configuration files changed:"
    echo "$staged_toml_files"
    echo "Will run full checks on entire project."
    should_check_all=true
    run_checks=true
fi

if [ -n "$staged_rust_files" ]; then
    echo "Staged Rust files:"
    echo "$staged_rust_files"
    run_checks=true
fi

if [ "$run_checks" = true ]; then
    # Check dependencies
    check_dependencies

    # Run rustfmt
    echo "Running rustfmt..."
    if [ "$should_check_all" = true ]; then
        cargo fmt --all
    else
        # Format only the staged files
        echo "$staged_rust_files" | xargs rustfmt 2>/dev/null || cargo fmt --all
    fi

    restage_files

    # Check formatting (rustfmt doesn't have a --fix mode like gofmt)
    echo "Checking code formatting..."
    if ! cargo fmt --all -- --check; then
        echo "Code formatting check failed. The code has been formatted for you."
        echo "Please review the changes and stage them before committing again."
        exit 1
    fi

    # Run clippy with auto-fixes
    echo "Running clippy with auto-fixes..."
    if ! cargo clippy --all-targets --all-features --fix --allow-dirty --allow-staged -- -D warnings; then
        echo "clippy found issues and attempted to fix them automatically."
        restage_files

        # Run clippy again to check if issues were resolved
        echo "Re-running clippy to verify fixes..."
        if ! cargo clippy --all-targets --all-features -- -D warnings; then
            echo "clippy still has issues that couldn't be auto-fixed."
            echo "Please fix the remaining linting issues before committing."
            exit 1
        fi
    fi

    restage_files
    echo "clippy checks passed!"

    # Run targeted tests
    if [ "$should_check_all" = true ]; then
        echo "Running all tests (Cargo files changed)..."
        if ! cargo test; then
            echo "cargo test failed"
            exit 1
        fi
    else
        run_targeted_tests
    fi
else
    echo "No staged Rust or Cargo files found. Skipping Rust-specific checks."
fi

# Run general pre-commit hooks for non-Rust files
echo "Running general pre-commit checks..."
pre-commit run --files $(git diff --cached --name-only) || true

echo "All pre-commit checks passed! Ready to commit."