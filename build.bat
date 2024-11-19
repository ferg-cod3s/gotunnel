@echo off
setlocal enabledelayedexpansion

:: Create bin directory if it doesn't exist
if not exist "bin" mkdir bin

:: Set version
set VERSION=v1.0.0

:: Build for Windows (amd64)
echo Building for Windows (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -o bin/gotunnel-%VERSION%-windows-amd64.exe ./cmd/gotunnel

:: Build for Linux (amd64)
echo Building for Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -o bin/gotunnel-%VERSION%-linux-amd64 ./cmd/gotunnel

:: Build for macOS (amd64)
echo Building for macOS (amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -o bin/gotunnel-%VERSION%-darwin-amd64 ./cmd/gotunnel

:: Build for macOS (arm64)
echo Building for macOS (arm64)...
set GOOS=darwin
set GOARCH=arm64
go build -o bin/gotunnel-%VERSION%-darwin-arm64 ./cmd/gotunnel

echo Build complete! Binaries are in the bin directory.
