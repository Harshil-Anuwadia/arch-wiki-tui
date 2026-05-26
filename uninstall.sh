#!/usr/bin/env bash

# archwiki-tui - Automated Uninstaller
# This script removes archwiki-tui from your system.

set -e

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

# Remove the binary
if [ -f "$INSTALL_DIR/$APP_NAME" ]; then
    info "Removing binary from $INSTALL_DIR/$APP_NAME..."
    rm "$INSTALL_DIR/$APP_NAME"
    success "Binary removed."
else
    info "Binary not found in $INSTALL_DIR. Skipping."
fi

# Ask to remove user data
echo -e "${BLUE}[QUESTION]${NC} Do you want to remove all user data (cache, history, config)? [y/N]"
read -r response
if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    info "Removing user data..."
    # Iterate through all users in /home to clean up their data
    for user_dir in /home/*; do
        if [ -d "$user_dir" ]; then
            rm -rf "$user_dir/.cache/archwiki-tui"
            rm -rf "$user_dir/.config/archwiki-tui"
        fi
    done
    # Also clean up for root if it exists
    rm -rf /root/.cache/archwiki-tui
    rm -rf /root/.config/archwiki-tui
    success "User data removed."
else
    info "User data preserved."
fi

success "archwiki-tui has been uninstalled successfully!"
