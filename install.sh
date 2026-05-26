#!/usr/bin/env bash

# archwiki-tui - Automated Installer
# ----------------------------------
# This script performs a full zero-labor installation of archwiki-tui.
# It automatically detects your distro and installs all missing dependencies.

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
    echo "  ▄▀█ █▀█ █▀▀ █░█ █   █ █ █▄▀ █"
    echo "  █▀█ █▀▄ █▄▄ █▀█ █▄█▄█ █ █░█ █"
    echo -e "${NC}"
    echo -e "  The definitive Arch Wiki terminal browser.\n"
}

# Check for root/sudo
if [ "$EUID" -ne 0 ]; then
    error "Please run this installer with sudo: curl ... | sudo bash"
fi

banner

# Detect Distro and install base tools
info "Detecting system environment..."
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
else
    DISTRO="unknown"
fi

install_pkg() {
    info "Installing missing dependency: $1..."
    case "$DISTRO" in
        arch|manjaro) pacman -S --needed --noconfirm "$@" ;;
        ubuntu|debian|kali|pop|linuxmint) 
            apt-get update -qq
            apt-get install -y -qq "$@" 
            ;;
        fedora|rhel|centos) dnf install -y -q "$@" ;;
        *) info "Unsupported distro for auto-install. Please install '$1' manually." ;;
    esac
}

# Ensure build tools are present
for cmd in git make gcc curl tar; do
    if ! command -v "$cmd" &> /dev/null; then
        pkg=$cmd
        if [ "$cmd" == "gcc" ]; then
            case "$DISTRO" in
                ubuntu|debian) pkg="build-essential" ;;
                fedora) pkg="gcc gcc-c++" ;;
            esac
        fi
        install_pkg "$pkg"
    fi
done

# Handle Go dependency
if ! command -v go &> /dev/null; then
    GO_VER="1.25.0"
    info "Go not found. Installing Go ${GO_VER}..."
    GO_ARCH="amd64"
    if [[ "$(uname -m)" == "aarch64" ]]; then GO_ARCH="arm64"; fi
    
    GO_TMP="/tmp/go${GO_VER}.linux-${GO_ARCH}.tar.gz"
    curl -fL "https://go.dev/dl/go${GO_VER}.linux-${GO_ARCH}.tar.gz" -o "$GO_TMP"
    
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "$GO_TMP"
    rm "$GO_TMP"
    export PATH=$PATH:/usr/local/go/bin
    success "Go installed to /usr/local/go."
fi

# Build from source
TMP_DIR=$(mktemp -d)
info "Cloning latest source code..."
git clone --depth 1 "$REPO_URL" "$TMP_DIR" &> /dev/null || error "Failed to clone repository."

cd "$TMP_DIR"

info "Building $APP_NAME..."
# Ensure the new Go is in PATH for the build subshell
export PATH=$PATH:/usr/local/go/bin
make build &> /dev/null || error "Build failed. Ensure 'make' and 'go' are functional."

# Final installation
info "Deploying binary to $INSTALL_DIR/$APP_NAME..."
cp "bin/$APP_NAME" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/$APP_NAME"

# Cleanup
info "Cleaning up..."
rm -rf "$TMP_DIR"

echo ""
success "Installation complete! Run '$APP_NAME' to start."
info "Tip: Run '$APP_NAME <query>' for a direct search."
