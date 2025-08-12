#!/bin/bash

# GitHub Secrets Setup Script for gotunnel
# This script helps you set up all required GitHub secrets for Apple code signing and other CI/CD needs

set -e

echo "🔐 GitHub Secrets Setup for gotunnel"
echo "===================================="
echo ""

# Check if gh CLI is available
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed."
    echo "Please install it first: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub CLI."
    echo "Please run: gh auth login"
    exit 1
fi

echo "✅ GitHub CLI is ready!"
echo ""

# Get repository info
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
echo "📦 Setting secrets for repository: $REPO"
echo ""

# Function to set a secret
set_secret() {
    local secret_name="$1"
    local description="$2"
    local is_file="$3"
    
    echo "🔑 Setting up: $secret_name"
    echo "   Description: $description"
    
    if [[ "$is_file" == "file" ]]; then
        echo "   Please provide the file path:"
        read -r file_path
        
        if [[ ! -f "$file_path" ]]; then
            echo "   ❌ File not found: $file_path"
            return 1
        fi
        
        # Convert file to base64
        if command -v base64 &> /dev/null; then
            secret_value=$(base64 -i "$file_path")
        else
            echo "   ❌ base64 command not found"
            return 1
        fi
    else
        echo "   Please enter the value (input will be hidden):"
        read -rs secret_value
    fi
    
    if [[ -z "$secret_value" ]]; then
        echo "   ⚠️  Empty value, skipping..."
        return 0
    fi
    
    # Set the secret
    echo "$secret_value" | gh secret set "$secret_name" --body
    echo "   ✅ Secret '$secret_name' set successfully!"
    echo ""
}

echo "🍎 Apple Code Signing Secrets"
echo "=============================="
echo ""

# Apple code signing secrets
set_secret "MACOS_CERTIFICATE" "Base64-encoded .p12 certificate file" "file"
set_secret "MACOS_CERTIFICATE_PWD" "Password for the .p12 certificate" "text"
set_secret "MACOS_SIGN_IDENTITY" "Certificate identity (e.g., 'Developer ID Application: Your Name (TEAM_ID)')" "text"
set_secret "KEYCHAIN_PASSWORD" "A secure password for the temporary keychain" "text"

echo "📱 Apple Notarization Secrets"
echo "=============================="
echo ""

set_secret "MACOS_NOTARY_ISSUER_ID" "App Store Connect Issuer ID" "text"
set_secret "MACOS_NOTARY_KEY_ID" "App Store Connect Key ID" "text"
set_secret "MACOS_NOTARY_KEY" "Base64-encoded .p8 private key content" "file"

echo "📦 Package Manager Secrets (Optional)"
echo "======================================"
echo ""

echo "Would you like to set up package manager secrets? (y/N)"
read -r setup_packages

if [[ "$setup_packages" =~ ^[Yy]$ ]]; then
    set_secret "HOMEBREW_TAP_TOKEN" "GitHub token for Homebrew tap repository" "text"
    set_secret "CHOCOLATEY_API_KEY" "Chocolatey API key for package publishing" "text"
    set_secret "CODECOV_TOKEN" "Codecov token for coverage reporting" "text"
fi

echo "🎉 GitHub Secrets Setup Complete!"
echo ""
echo "📋 Summary of secrets that should be set:"
echo ""
echo "   Required for Apple Code Signing:"
echo "   ✓ MACOS_CERTIFICATE"
echo "   ✓ MACOS_CERTIFICATE_PWD"
echo "   ✓ MACOS_SIGN_IDENTITY"
echo "   ✓ KEYCHAIN_PASSWORD"
echo ""
echo "   Required for Apple Notarization:"
echo "   ✓ MACOS_NOTARY_ISSUER_ID"
echo "   ✓ MACOS_NOTARY_KEY_ID"
echo "   ✓ MACOS_NOTARY_KEY"
echo ""
echo "   Optional for package managers:"
echo "   • HOMEBREW_TAP_TOKEN"
echo "   • CHOCOLATEY_API_KEY"
echo "   • CODECOV_TOKEN"
echo ""
echo "🔍 You can verify your secrets with:"
echo "   gh secret list"
echo ""
echo "📚 For more information, see:"
echo "   • APPLE_SIGNING_SETUP.md"
echo "   • https://docs.github.com/en/actions/security-guides/encrypted-secrets"