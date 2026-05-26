#!/usr/bin/env bash

# archwiki-tui - Automated Uninstaller
# ------------------------------------
# This script removes archwiki-tui from your system.

set -euo pipefail

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
    echo -e "Uninstalling archwiki-tui...\n"
}

# Check for root/sudo
if [ "$EUID" -ne 0 ]; then
    error "Please run this uninstaller with sudo: sudo ./uninstall.sh"
fi

banner

# Remove binary
if [ -f "$INSTALL_DIR/$APP_NAME" ]; then
    info "Removing binary: $INSTALL_DIR/$APP_NAME"
    rm "$INSTALL_DIR/$APP_NAME"
    success "Binary removed."
else
    info "Binary not found in $INSTALL_DIR. Skipping."
fi

# Clean up user data
echo -e "${BLUE}[QUESTION]${NC} Do you want to remove all local data (cache, history, config)? [y/N]"
read -r response < /dev/tty || response="n"
if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    info "Cleaning up data for all users..."
    for user_dir in /home/*; do
        if [ -d "$user_dir" ]; then
            rm -rf "$user_dir/.cache/archwiki-tui"
            rm -rf "$user_dir/.config/archwiki-tui"
        fi
    done
    rm -rf /root/.cache/archwiki-tui
    rm -rf /root/.config/archwiki-tui
    success "Local data removed."
else
    info "Local data preserved."
fi

echo ""
success "archwiki-tui has been uninstalled."
