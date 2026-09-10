#!/usr/bin/env bash
#
# oh-my-logs (oml) — Cross-Platform Release Packaging Script
#
# Builds production binaries for macOS, Linux, and Windows, packages them
# with README, LICENSE, and starter profiles, and generates SHA-256 checksums.
#
set -e

RESET="\033[0m"
BOLD="\033[1m"
GREEN="\033[32m"
BLUE="\033[34m"
YELLOW="\033[33m"
CYAN="\033[36m"
RED="\033[31m"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$SCRIPT_DIR/dist"

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

# 1. Determine version and codename
MAIN_GO="$SCRIPT_DIR/cmd/oml/main.go"
if [ ! -f "$MAIN_GO" ]; then
    print_error "Could not find $MAIN_GO"
    exit 1
fi

VERSION_FROM_FILE="$(grep -E 'version\s*=\s*"[^"]+"' "$MAIN_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"
CODENAME_FROM_FILE="$(grep -E 'codename\s*=\s*"[^"]+"' "$MAIN_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"

VERSION="${1:-$VERSION_FROM_FILE}"
if [[ "$VERSION" != v* ]]; then
    TAG="v$VERSION"
else
    TAG="$VERSION"
    VERSION="${VERSION#v}"
fi

CODENAME="${CODENAME_FROM_FILE:-Release}"

printf "\n${BOLD}${CYAN}Packaging oh-my-logs ($TAG — $CODENAME)${RESET}\n"
printf "Output Directory: %s\n\n" "$DIST_DIR"

# 2. Verify Go compiler
if ! command -v go >/dev/null 2>&1; then
    print_error "Go compiler not found. Please install Go (https://go.dev)."
    exit 1
fi

# 3. Clean and initialize dist directory
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# 4. Target platforms: (GOOS GOARCH EXT ARCHIVE_TYPE)
TARGETS=(
    "darwin arm64  \"\"   tar.gz"
    "darwin amd64  \"\"   tar.gz"
    "linux  amd64  \"\"   tar.gz"
    "linux  arm64  \"\"   tar.gz"
    "windows amd64 .exe  zip"
    "windows arm64 .exe  zip"
)

LDFLAGS="-s -w"

print_step "Cross-compiling binaries with optimizations (-ldflags=\"$LDFLAGS\")..."

TMP_STAGE="$(mktemp -d -t oml-release-XXXXXX)"
trap 'rm -rf "$TMP_STAGE"' EXIT

for TARGET in "${TARGETS[@]}"; do
    eval "PARSED=($TARGET)"
    GOOS="${PARSED[0]}"
    GOARCH="${PARSED[1]}"
    EXE_EXT="${PARSED[2]}"
    ARCHIVE_EXT="${PARSED[3]}"

    TARGET_NAME="oml_${TAG}_${GOOS}_${GOARCH}"
    PACKAGE_DIR="$TMP_STAGE/$TARGET_NAME"
    mkdir -p "$PACKAGE_DIR/profiles"

    BINARY_NAME="oml${EXE_EXT}"
    BINARY_DEST="$PACKAGE_DIR/$BINARY_NAME"

    printf "  • Building %-8s %-6s -> %s\n" "$GOOS" "$GOARCH" "$TARGET_NAME.$ARCHIVE_EXT"

    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
        go build -trimpath -ldflags="$LDFLAGS" -o "$BINARY_DEST" "$SCRIPT_DIR/cmd/oml"

    # Copy documentation & licenses
    [ -f "$SCRIPT_DIR/README.md" ] && cp "$SCRIPT_DIR/README.md" "$PACKAGE_DIR/"
    [ -f "$SCRIPT_DIR/LICENSE" ] && cp "$SCRIPT_DIR/LICENSE" "$PACKAGE_DIR/"

    # Copy example profiles
    if [ -d "$SCRIPT_DIR/profiles/examples" ]; then
        cp -R "$SCRIPT_DIR/profiles/examples" "$PACKAGE_DIR/profiles/"
    fi

    # Create archive
    ARCHIVE_PATH="$DIST_DIR/$TARGET_NAME.$ARCHIVE_EXT"
    if [ "$ARCHIVE_EXT" = "tar.gz" ]; then
        (cd "$TMP_STAGE" && tar -czf "$ARCHIVE_PATH" "$TARGET_NAME")
    elif [ "$ARCHIVE_EXT" = "zip" ]; then
        (cd "$TMP_STAGE" && zip -q -r "$ARCHIVE_PATH" "$TARGET_NAME")
    fi
done

print_success "All binaries compiled and packaged successfully."

# 5. Generate SHA-256 Checksums
print_step "Generating SHA-256 checksums..."
CHECKSUM_FILE="$DIST_DIR/checksums.txt"

(
    cd "$DIST_DIR"
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 oml_* > checksums.txt
    elif command -v sha256sum >/dev/null 2>&1; then
        sha256sum oml_* > checksums.txt
    fi
)

print_success "Generated $CHECKSUM_FILE"

# 6. Print Summary
printf "\n${BOLD}${GREEN}======================================================${RESET}\n"
printf "${BOLD}${GREEN} Release Artifacts Ready in ./dist/${RESET}\n"
printf "${BOLD}${GREEN}======================================================${RESET}\n\n"

ls -lh "$DIST_DIR" | tail -n +2 | awk '{printf "  %-10s  %s\n", $5, $9}'

printf "\n${BOLD}Next steps to publish release ${CYAN}${TAG}${RESET}:\n\n"
printf "1. Tag the release in git:\n"
printf "   git tag -a %s -m \"Release %s — %s\"\n" "$TAG" "$TAG" "$CODENAME"
printf "   git push origin %s\n\n" "$TAG"

if command -v gh >/dev/null 2>&1; then
    printf "2. Create GitHub Release via gh CLI:\n"
    printf "   gh release create %s ./dist/* \\\n" "$TAG"
    printf "      --title \"%s — %s\" \\\n" "$TAG" "$CODENAME"
    printf "      --notes-file pr.md\n\n"
else
    printf "2. Upload artifacts to GitHub manually:\n"
    printf "   Visit https://github.com/brenonsantos/oh-my-logs/releases/new\n"
    printf "   Drag and drop all files from ./dist/ and paste release notes.\n\n"
fi
