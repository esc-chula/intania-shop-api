@echo off
REM Development Environment Setup Script for Windows
REM This script installs pre-commit hooks and sets up the development environment

echo Setting up development environment for Intania Shop API...

REM Check if pre-commit is installed
where pre-commit >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo pre-commit is already installed
) else (
    echo Installing pre-commit...

    REM Try different installation methods
    where choco >nul 2>nul
    if %ERRORLEVEL% EQU 0 (
        echo Using Chocolatey to install pre-commit...
        choco install pre-commit
    ) else (
        where python >nul 2>nul
        if %ERRORLEVEL% EQU 0 (
            echo Using pip to install pre-commit...
            pip install pre-commit --user
            REM Add Python user scripts to PATH for this session
            for /f "tokens=*" %%i in ('python -m site --user-site') do set SITE=%%i
            set PATH=%PATH%;%SITE%\..\Scripts
        ) else (
            where python3 >nul 2>nul
            if %ERRORLEVEL% EQU 0 (
                echo Using pip3 to install pre-commit...
                pip3 install pre-commit --user
                REM Add Python user scripts to PATH for this session
                for /f "tokens=*" %%i in ('python3 -m site --user-site') do set SITE=%%i
                set PATH=%PATH%;%SITE%\..\Scripts
            ) else (
                echo Error: Could not find choco, pip, or pip3.
                echo Please install pre-commit manually:
                echo   choco install pre-commit
                echo   or pip install pre-commit
                echo   or visit: https://pre-commit.com/#installation
                pause
                exit /b 1
            )
        )
    )
)

echo Installing pre-commit hooks...
pre-commit install

echo Verifying pre-commit installation...
pre-commit --version

echo.
echo Development environment setup complete!
echo.
echo Pre-commit hooks are now active and will run on:
echo    - git commit
echo    - git push
echo.
echo You can also run hooks manually:
echo    pre-commit run --all-files  # Run on all files
echo    make lint                   # Run formatting and clippy
echo    make test                   # Run all tests
echo.
pause
