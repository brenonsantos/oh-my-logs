#!/usr/bin/env bash
#
# oh-my-logs (oml) — Cross-platform installer for macOS and Linux
#
set -e

RESET="\033[0m"
BOLD="\033[1m"
GREEN="\033[32m"
BLUE="\033[34m"
YELLOW="\033[33m"
RED="\033[31m"

print_step() {
    printf "${BLUE}${BOLD}==>${RESET} %s\n" "$1"
}

print_success() {
    printf "${GREEN}${BOLD}✓${RESET} %s\n" "$1"
}

print_warn() {
    printf "${YELLOW}${BOLD}!${RESET} %s\n" "$1"
}

print_error() {
    printf "${RED}${BOLD}✗${RESET} %s\n" "$1" >&2
}

# Determine OS
OS="$(uname -s)"
case "$OS" in
    Darwin)
        OS_NAME="macOS"
        PROFILES_DIR="$HOME/Library/Application Support/oh-my-logs/profiles"
        ;;
    Linux)
        OS_NAME="Linux"
        PROFILES_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/oh-my-logs/profiles"
        ;;
    *)
        print_error "Unsupported operating system: $OS"
        printf "For Windows, please run install.ps1 in PowerShell.\n"
        exit 1
        ;;
esac

# Handle --uninstall
if [ "$1" = "--uninstall" ] || [ "$1" = "-u" ]; then
    print_step "Uninstalling oh-my-logs (oml)..."
    REMOVED=0
    for DIR in "/usr/local/bin" "$HOME/.local/bin" "$HOME/bin"; do
        if [ -f "$DIR/oml" ]; then
            if [ -w "$DIR" ]; then
                rm -f "$DIR/oml"
            else
                sudo rm -f "$DIR/oml"
            fi
            print_success "Removed $DIR/oml"
            REMOVED=1
        fi
    done
    if [ "$REMOVED" -eq 0 ]; then
        print_warn "No oml binary found in standard PATH directories."
    fi
    printf "\nNote: Profiles in %s were preserved.\n" "$PROFILES_DIR"
    exit 0
fi

printf "${BOLD}oh-my-logs (oml) Installer for %s${RESET}\n\n" "$OS_NAME"

# Determine target directory
if [ -w "/usr/local/bin" ]; then
    TARGET_DIR="/usr/local/bin"
elif command -v sudo >/dev/null 2>&1 && [ -t 0 ]; then
    # Interactive session with sudo available
    TARGET_DIR="/usr/local/bin"
    USE_SUDO=1
else
    TARGET_DIR="$HOME/.local/bin"
    mkdir -p "$TARGET_DIR"
fi

# Build or locate binary
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH=""

if [ -f "$SCRIPT_DIR/oml" ]; then
    BIN_PATH="$SCRIPT_DIR/oml"
elif command -v go >/dev/null 2>&1; then
    print_step "Building oml binary from source..."
    (cd "$SCRIPT_DIR" && go build -o oml ./cmd/oml)
    BIN_PATH="$SCRIPT_DIR/oml"
else
    print_error "Neither prebuilt 'oml' binary nor 'go' compiler found."
    printf "Please install Go (https://go.dev) or build the binary first.\n"
    exit 1
fi

# Install binary
print_step "Installing oml to $TARGET_DIR/oml..."
if [ "$USE_SUDO" = "1" ]; then
    sudo cp "$BIN_PATH" "$TARGET_DIR/oml"
    sudo chmod 755 "$TARGET_DIR/oml"
else
    mkdir -p "$TARGET_DIR"
    cp "$BIN_PATH" "$TARGET_DIR/oml"
    chmod 755 "$TARGET_DIR/oml"
fi
print_success "Installed binary at $TARGET_DIR/oml"

# Install default example profiles
mkdir -p "$PROFILES_DIR"
if [ -d "$SCRIPT_DIR/profiles/examples" ]; then
    COPIED=0
    for f in "$SCRIPT_DIR/profiles/examples"/*.yaml; do
        [ -e "$f" ] || continue
        base="$(basename "$f")"
        if [ ! -f "$PROFILES_DIR/$base" ]; then
            cp "$f" "$PROFILES_DIR/$base"
            COPIED=$((COPIED + 1))
        fi
    done
    if [ "$COPIED" -gt 0 ]; then
        print_success "Copied $COPIED default profile(s) to $PROFILES_DIR"
    fi
fi

# Check PATH
printf "\n"
if ! command -v oml >/dev/null 2>&1; then
    print_warn "$TARGET_DIR is not currently in your PATH."
    printf "Add it to your shell configuration file (~/.zshrc or ~/.bashrc):\n\n"
    printf "    export PATH=\"%s:\$PATH\"\n\n" "$TARGET_DIR"
    printf "Then restart your terminal or run:\n"
    printf "    source ~/.zshrc  # or ~/.bashrc\n\n"
fi

print_success "oh-my-logs is ready to use!"
printf "\nTry running:\n"
printf "    oml                 # Open TUI serial monitor\n"
printf "    oml --list-profiles # View available log profiles\n"
printf "    oml --help          # See all options\n\n"
