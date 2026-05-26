#!/usr/bin/env bash

# archwiki-tui - Automated Installer
# This script installs archwiki-tui to your system.

set -e

REPO_URL="https://github.com/Harshil-Anuwadia/arch-wiki-tui.git"
INSTALL_DIR="/usr/local/bin"
APP_NAME="archwiki"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Check for sudo/root permissions
if [ "$EUID" -ne 0 ]; then
    error "Please run as root or using sudo."
fi

# Check for dependencies
info "Checking dependencies..."
for cmd in git go curl tar; do
    if ! command -v $cmd &> /dev/null; then
        if [ "$cmd" == "go" ]; then
            info "Go not found. Attempting to install Go in /usr/local..."
            GO_VER="1.25.0"
            curl -fsSL "https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
            tar -C /usr/local -xzf /tmp/go.tar.gz
            export PATH=$PATH:/usr/local/go/bin
            success "Go ${GO_VER} installed."
        else
            error "$cmd is not installed. Please install it and try again."
        fi
    fi
done

# Temporary directory for cloning
TMP_DIR=$(mktemp -d)
info "Cloning repository..."
git clone --depth 1 "$REPO_URL" "$TMP_DIR" || error "Failed to clone repository."

cd "$TMP_DIR"

# Build the project
info "Building $APP_NAME..."
make build || error "Failed to build $APP_NAME."

# Install the binary
info "Installing binary to $INSTALL_DIR/$APP_NAME..."
cp "bin/$APP_NAME" "$INSTALL_DIR/" || error "Failed to copy binary."
chmod +x "$INSTALL_DIR/$APP_NAME"

# Cleanup
info "Cleaning up temporary files..."
rm -rf "$TMP_DIR"

success "archwiki-tui has been installed successfully!"
info "Run it by typing: $APP_NAME"
