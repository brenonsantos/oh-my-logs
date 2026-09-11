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
        GOOS="darwin"
        PROFILES_DIR="$HOME/Library/Application Support/oh-my-logs/profiles"
        ;;
    Linux)
        OS_NAME="Linux"
        GOOS="linux"
        PROFILES_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/oh-my-logs/profiles"
        ;;
    *)
        print_error "Unsupported operating system: $OS"
        printf "For Windows, please run install.ps1 in PowerShell.\n"
        exit 1
        ;;
esac

# Determine architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)
        GOARCH="amd64"
        ;;
    arm64|aarch64)
        GOARCH="arm64"
        ;;
    *)
        print_error "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Parse arguments
DO_UNINSTALL=0
NIGHTLY=0
SPECIFIED_TAG="${VERSION:-${TAG:-}}"

while [ $# -gt 0 ]; do
    case "$1" in
        --uninstall|-u)
            DO_UNINSTALL=1
            shift
            ;;
        --nightly|-n)
            NIGHTLY=1
            shift
            ;;
        --version|-v|--tag)
            SPECIFIED_TAG="$2"
            shift 2
            ;;
        --version=*|--tag=*)
            SPECIFIED_TAG="${1#*=}"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# Handle --uninstall
if [ "$DO_UNINSTALL" = "1" ]; then
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

if [ "$NIGHTLY" = "1" ]; then
    printf "${BOLD}oh-my-logs (oml) Installer [Nightly Channel] for %s (%s)${RESET}\n\n" "$OS_NAME" "$GOARCH"
elif [ -n "$SPECIFIED_TAG" ]; then
    printf "${BOLD}oh-my-logs (oml) Installer [%s] for %s (%s)${RESET}\n\n" "$SPECIFIED_TAG" "$OS_NAME" "$GOARCH"
else
    printf "${BOLD}oh-my-logs (oml) Installer for %s (%s)${RESET}\n\n" "$OS_NAME" "$GOARCH"
fi

# Determine target directory
if [ -w "/usr/local/bin" ]; then
    TARGET_DIR="/usr/local/bin"
elif [ -f "/usr/local/bin/oml" ] && command -v sudo >/dev/null 2>&1; then
    # An existing binary in /usr/local/bin should be updated directly with sudo
    TARGET_DIR="/usr/local/bin"
    USE_SUDO=1
elif command -v sudo >/dev/null 2>&1 && [ -t 0 ]; then
    TARGET_DIR="/usr/local/bin"
    USE_SUDO=1
else
    TARGET_DIR="$HOME/.local/bin"
    mkdir -p "$TARGET_DIR"
fi

# Locate, build, or download binary
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || true)"
BIN_PATH=""
EXTRACTED_PROFILES=""

if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/oml" ]; then
    print_step "Using local binary at $SCRIPT_DIR/oml..."
    BIN_PATH="$SCRIPT_DIR/oml"
elif [ -n "$SCRIPT_DIR" ] && [ -d "$SCRIPT_DIR/cmd/oml" ] && command -v go >/dev/null 2>&1; then
    print_step "Building oml binary from source..."
    (cd "$SCRIPT_DIR" && go build -ldflags="-s -w" -o oml ./cmd/oml)
    BIN_PATH="$SCRIPT_DIR/oml"
else
    # Download prebuilt binary from GitHub Releases
    REPO="brenonsantos/oh-my-logs"
    TMP_DIR="$(mktemp -d -t oml-install-XXXXXX)"
    trap 'rm -rf "$TMP_DIR"' EXIT

    if [ "$NIGHTLY" = "1" ]; then
        RELEASE_NAME="nightly build"
        RELEASE_URL="https://api.github.com/repos/$REPO/releases/tags/nightly"
    elif [ -n "$SPECIFIED_TAG" ]; then
        RELEASE_NAME="release $SPECIFIED_TAG"
        RELEASE_URL="https://api.github.com/repos/$REPO/releases/tags/$SPECIFIED_TAG"
    else
        RELEASE_NAME="latest stable release"
        RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"
    fi

    print_step "Fetching $RELEASE_NAME for $OS_NAME ($GOARCH) from GitHub..."
    ASSET_URL=""
    if command -v curl >/dev/null 2>&1; then
        JSON="$(curl -fsSL -H "User-Agent: oh-my-logs-installer" "$RELEASE_URL" 2>/dev/null || true)"
        ASSET_URL="$(printf "%s" "$JSON" | grep -o "https://[^\"]*_${GOOS}_${GOARCH}\.tar\.gz" | head -n 1 || true)"
    fi

    if [ -z "$ASSET_URL" ]; then
        print_error "Could not find prebuilt release for ${GOOS}_${GOARCH} on GitHub."
        printf "Please install Go (https://go.dev) to build from source.\n"
        exit 1
    fi

    print_step "Downloading $(basename "$ASSET_URL")..."
    if ! curl -fsSL -o "$TMP_DIR/oml.tar.gz" "$ASSET_URL"; then
        print_error "Failed to download $ASSET_URL"
        exit 1
    fi

    if ! gzip -t "$TMP_DIR/oml.tar.gz" 2>/dev/null; then
        print_error "Downloaded archive is corrupted or not a valid gzip file."
        exit 1
    fi

    print_step "Extracting archive..."
    tar -xzf "$TMP_DIR/oml.tar.gz" -C "$TMP_DIR"

    BIN_PATH="$(find "$TMP_DIR" -name "oml" -type f | head -n 1)"
    if [ ! -f "$BIN_PATH" ]; then
        print_error "Extracted archive did not contain 'oml' binary."
        exit 1
    fi
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

if [ "$TARGET_DIR" != "/usr/local/bin" ] && [ -f "/usr/local/bin/oml" ]; then
    print_warn "An existing binary at /usr/local/bin/oml may take precedence over $TARGET_DIR/oml in your PATH."
    printf "To update /usr/local/bin/oml, run: sudo cp \"%s/oml\" /usr/local/bin/oml\n\n" "$TARGET_DIR"
fi

# Ensure profiles directory exists
mkdir -p "$PROFILES_DIR"

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
