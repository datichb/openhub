#!/bin/sh
# install.sh — Install oh (OpenHub CLI) binary
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | sh
#   curl -sSfL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | sh -s -- --dir /custom/path
#
# Options:
#   --dir <path>    Install directory (default: /usr/local/bin)
#   --version <v>   Specific version to install (default: latest)

set -e

REPO="datichb/openhub"
BINARY_NAME="oh"
DEFAULT_INSTALL_DIR="/usr/local/bin"

# --- Parse arguments ---
INSTALL_DIR="$DEFAULT_INSTALL_DIR"
VERSION=""

while [ $# -gt 0 ]; do
    case "$1" in
        --dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# --- Detect OS and architecture ---
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin) OS="darwin" ;;
        linux)  OS="linux" ;;
        *)
            echo "Error: Unsupported operating system: $OS"
            echo "oh supports: macOS (darwin), Linux"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *)
            echo "Error: Unsupported architecture: $ARCH"
            echo "oh supports: x86_64/amd64, arm64/aarch64"
            exit 1
            ;;
    esac

    echo "${OS}_${ARCH}"
}

# --- Get latest version ---
get_latest_version() {
    if command -v curl > /dev/null 2>&1; then
        curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
    elif command -v wget > /dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
    else
        echo "Error: curl or wget is required" >&2
        exit 1
    fi
}

# --- Download file ---
download() {
    URL="$1"
    DEST="$2"
    if command -v curl > /dev/null 2>&1; then
        curl -sSfL -o "$DEST" "$URL"
    elif command -v wget > /dev/null 2>&1; then
        wget -qO "$DEST" "$URL"
    fi
}

# --- Main ---
main() {
    PLATFORM=$(detect_platform)

    if [ -z "$VERSION" ]; then
        echo "Fetching latest version..."
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            echo "Error: Could not determine latest version."
            echo "Try specifying a version: --version 2.0.0"
            exit 1
        fi
    fi

    ARCHIVE_NAME="${BINARY_NAME}_${PLATFORM}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE_NAME}"

    echo "Installing oh v${VERSION} (${PLATFORM})..."
    echo "  From: ${DOWNLOAD_URL}"
    echo "  To:   ${INSTALL_DIR}/${BINARY_NAME}"
    echo ""

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    # Download
    echo "Downloading..."
    download "$DOWNLOAD_URL" "$TMP_DIR/$ARCHIVE_NAME"

    # Verify download
    if [ ! -f "$TMP_DIR/$ARCHIVE_NAME" ]; then
        echo "Error: Download failed."
        echo ""
        echo "Please check:"
        echo "  - Version v${VERSION} exists: https://github.com/${REPO}/releases"
        echo "  - Your internet connection"
        exit 1
    fi

    # Extract
    echo "Extracting..."
    tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"

    # Verify binary
    if [ ! -f "$TMP_DIR/$BINARY_NAME" ]; then
        echo "Error: Binary not found in archive."
        exit 1
    fi

    # Install
    echo "Installing..."
    mkdir -p "$INSTALL_DIR"
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        chmod 755 "$INSTALL_DIR/$BINARY_NAME"
    else
        echo "  (requires sudo for $INSTALL_DIR)"
        sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        sudo chmod 755 "$INSTALL_DIR/$BINARY_NAME"
    fi

    # Verify installation
    if command -v "$BINARY_NAME" > /dev/null 2>&1; then
        echo ""
        echo "Successfully installed oh v${VERSION}!"
        echo ""
        "$BINARY_NAME" version
    else
        echo ""
        echo "Installed oh to: ${INSTALL_DIR}/${BINARY_NAME}"
        echo ""
        echo "Make sure ${INSTALL_DIR} is in your PATH:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
}

main
