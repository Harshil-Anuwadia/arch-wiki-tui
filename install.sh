#!/usr/bin/env bash

# archwiki-tui - Automated Installer
# ----------------------------------
# This script installs archwiki-tui to your system.

set -euo pipefail

REPO_URL="https://github.com/Harshil-Anuwadia/arch-wiki-tui.git"
APP_NAME="archwiki"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; exit 1; }

banner() {
    echo -e "${BLUE}${BOLD}"
    echo "    ___                __               _ __   _ "
    echo "   /   |  _____________/ /_ _      _(_/ /__(_)"
    echo "  / /| | / ___/ ___/ __ \ | /| / / / //_/ / "
    echo " / ___ |/ /  / /__/ / / / |/ |/ / / / ,< / /  "
    echo "/_/  |_/_/   \___/_/ /_/\__/|__/_/_/_/|_/_/   "
    echo -e "${NC}"
    echo -e "Installing the definitive Arch Wiki terminal browser...\n"
}

# Check for root/sudo
if [ "$EUID" -ne 0 ]; then
    error "Please run this installer with sudo: curl ... | sudo bash"
fi

banner

# Verify core dependencies
info "Verifying system dependencies..."
for cmd in git curl tar; do
    if ! command -v "$cmd" &> /dev/null; then
        error "$cmd is not installed. Please install it and try again."
    fi
done

# Handle Go dependency
if ! command -v go &> /dev/null; then
    info "Go not found. Installing Go 1.25.0 to /usr/local/go..."
    GO_ARCH="amd64"
    if [[ "$(uname -m)" == "aarch64" ]]; then GO_ARCH="arm64"; fi
    
    GO_TMP="/tmp/go1.25.0.linux-${GO_ARCH}.tar.gz"
    curl -fL "https://go.dev/dl/go1.25.0.linux-${GO_ARCH}.tar.gz" -o "$GO_TMP"
    
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "$GO_TMP"
    export PATH=$PATH:/usr/local/go/bin
    rm "$GO_TMP"
    success "Go installed successfully."
fi

# Build from source
TMP_DIR=$(mktemp -d)
info "Cloning latest source code..."
git clone --depth 1 "$REPO_URL" "$TMP_DIR" &> /dev/null || error "Failed to clone repository."

cd "$TMP_DIR"

info "Building $APP_NAME (v$(cat VERSION))..."
# Use the local go if we just installed it
export PATH=$PATH:/usr/local/go/bin
make build &> /dev/null || error "Build failed. Check your Go environment."

# Final installation
info "Deploying binary to $INSTALL_DIR/$APP_NAME..."
cp "bin/$APP_NAME" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/$APP_NAME"

# Cleanup
info "Cleaning up..."
rm -rf "$TMP_DIR"

echo ""
success "Installation complete! You can now run '$APP_NAME'."
info "Type '$APP_NAME --help' to get started."
