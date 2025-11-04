#!/bin/sh
#
# mcp-k6 installer
#
# This script installs the mcp-k6 MCP server binary on Linux and macOS.
# It follows best practices for safe, audit-friendly installation scripts.
#
# Usage:
#   sh install.sh
#   curl -fsSL https://raw.githubusercontent.com/grafana/mcp-k6/main/install.sh | sh
#
# Environment variables:
#   MCP_K6_DIR            Custom installation directory (default: auto-detect)
#   MCP_K6_SYSTEM_INSTALL Set to 1 to install to /usr/local/bin (requires sudo)
#   MCP_K6_NO_PATH_UPDATE Set to 1 to skip PATH modification (for CI/automation)
#

set -e
set -u

# Configuration
REPO="grafana/mcp-k6"
BINARY_NAME="mcp-k6"
GITHUB_API="https://api.github.com/repos/${REPO}"

# Colors for output (only if terminal supports it)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    NC=''
fi

# Cleanup function
cleanup() {
    if [ -n "${TEMP_DIR:-}" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}

trap cleanup EXIT INT TERM

# Print functions
info() {
    printf "%b\n" "$1"
}

success() {
    printf "%b%s%b\n" "$GREEN" "$1" "$NC"
}

error() {
    printf "%b%s%b\n" "$RED" "$1" "$NC" >&2
}

warning() {
    printf "%b%s%b\n" "$YELLOW" "$1" "$NC"
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check for required dependencies
check_dependencies() {
    local missing=""

    # Check for download tool (curl or wget)
    if ! command_exists curl && ! command_exists wget; then
        missing="${missing}- curl or wget\n"
    fi

    # Check for tar
    if ! command_exists tar; then
        missing="${missing}- tar\n"
    fi

    # Check for checksum utility
    if ! command_exists sha256sum && ! command_exists shasum; then
        missing="${missing}- sha256sum or shasum\n"
    fi

    if [ -n "$missing" ]; then
        error "Error: Missing required dependencies:"
        printf "%b" "$missing"
        exit 1
    fi
}

# Detect platform and architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux*)
            OS="linux"
            ;;
        Darwin*)
            OS="darwin"
            ;;
        *)
            error "Error: Unsupported operating system: $OS"
            error "Supported: Linux, macOS"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64)
            ARCH="arm64"
            ;;
        arm64)
            ARCH="arm64"
            ;;
        *)
            error "Error: Unsupported architecture: $ARCH"
            error "Supported: x86_64 (amd64), aarch64/arm64"
            exit 1
            ;;
    esac

    info "Detected platform: ${OS}_${ARCH}"
}

# Determine installation directory
determine_install_dir() {
    # Check for custom directory via environment variable
    if [ -n "${MCP_K6_DIR:-}" ]; then
        INSTALL_DIR="$MCP_K6_DIR"
        CUSTOM_INSTALL=1
        info "Using custom installation directory: $INSTALL_DIR"
        return
    fi

    # Check for system-wide installation request
    if [ "${MCP_K6_SYSTEM_INSTALL:-0}" = "1" ]; then
        INSTALL_DIR="/usr/local/bin"
        NEEDS_SUDO=1
        info "System-wide installation requested: $INSTALL_DIR"
        return
    fi

    # Auto-detect best user directory
    NEEDS_SUDO=0
    CUSTOM_INSTALL=0

    if [ -d "$HOME/.local/bin" ] && echo "$PATH" | grep -q "$HOME/.local/bin"; then
        INSTALL_DIR="$HOME/.local/bin"
        info "Installing to: $INSTALL_DIR (already in PATH)"
    elif [ -d "$HOME/bin" ] && echo "$PATH" | grep -q "$HOME/bin"; then
        INSTALL_DIR="$HOME/bin"
        info "Installing to: $INSTALL_DIR (already in PATH)"
    else
        INSTALL_DIR="$HOME/.mcp-k6/bin"
        NEEDS_PATH_UPDATE=1
        info "Installing to: $INSTALL_DIR"
    fi
}

# Check sudo availability and permissions
check_sudo() {
    if [ "$NEEDS_SUDO" = "0" ]; then
        return 0
    fi

    if ! command_exists sudo; then
        error "Error: sudo is required for system-wide installation but not found"
        exit 1
    fi

    # Check for passwordless sudo
    if sudo -n -v >/dev/null 2>&1; then
        info "Passwordless sudo available"
        return 0
    fi

    # Check if we're in an interactive terminal
    if [ -t 0 ]; then
        warning "This script will need to run 'sudo mv' to install to $INSTALL_DIR"
        printf "Please enter your password when prompted.\n"
        return 0
    else
        error "Error: Non-interactive session and no passwordless sudo available"
        error "Cannot install to $INSTALL_DIR without sudo"
        exit 1
    fi
}

# Download file using curl or wget
download_file() {
    local url="$1"
    local output="$2"

    if command_exists curl; then
        if [ "$output" = "-" ]; then
            curl -fsSL "$url"
        else
            curl -fsSL "$url" -o "$output"
        fi
    elif command_exists wget; then
        if [ "$output" = "-" ]; then
            wget -q -O - "$url"
        else
            wget -q -O "$output" "$url"
        fi
    else
        error "Error: Neither curl nor wget is available"
        exit 1
    fi
}

# Verify checksum
verify_checksum() {
    local file="$1"
    local expected_checksum="$2"
    local actual_checksum

    if command_exists sha256sum; then
        actual_checksum="$(sha256sum "$file" | awk '{print $1}')"
    elif command_exists shasum; then
        actual_checksum="$(shasum -a 256 "$file" | awk '{print $1}')"
    else
        error "Error: No checksum utility available"
        exit 1
    fi

    if [ "$actual_checksum" != "$expected_checksum" ]; then
        error "Error: Checksum verification failed!"
        error "Expected: $expected_checksum"
        error "Got:      $actual_checksum"
        exit 1
    fi

    success "✓ Checksum verified"
}

# Get latest release tag from GitHub API
get_latest_release_tag() {
    info "Fetching latest release information..."

    # Fetch release info and extract tag name
    # This avoids the need for jq by using simple text processing
    LATEST_TAG=$(download_file "${GITHUB_API}/releases/latest" - 2>/dev/null | grep '"tag_name"' | head -n 1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')

    if [ -z "$LATEST_TAG" ]; then
        error "Error: Could not determine latest release version"
        error "This might mean:"
        error "  - No published releases are available yet"
        error "  - Network connectivity issues"
        error "  - GitHub API rate limiting"
        exit 1
    fi

    info "Latest release: $LATEST_TAG"
}

# Check for existing installation
check_existing_installation() {
    if command_exists "$BINARY_NAME"; then
        EXISTING_PATH="$(command -v "$BINARY_NAME")"
        EXISTING_VERSION="$("$BINARY_NAME" --version 2>/dev/null | head -n 1 || echo "unknown")"
        info "Found existing installation: $EXISTING_VERSION"
        info "Location: $EXISTING_PATH"
        return 0
    fi
    return 1
}

# Download and verify the binary
download_and_verify() {
    TEMP_DIR="$(mktemp -d)"
    cd "$TEMP_DIR"

    # Construct download URLs using the release tag
    ARCHIVE_NAME="${BINARY_NAME}_${LATEST_TAG}_${OS}_${ARCH}.tar.gz"
    ARCHIVE_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${ARCHIVE_NAME}"
    CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${BINARY_NAME}_checksums.txt"

    info ""
    info "Downloading from: $ARCHIVE_URL"

    # Download archive
    if ! download_file "$ARCHIVE_URL" "$ARCHIVE_NAME"; then
        error "Error: Failed to download binary"
        error "URL: $ARCHIVE_URL"
        error ""
        error "This might mean:"
        error "  - Network connectivity issues"
        error "  - The release doesn't include binaries for ${OS}_${ARCH}"
        exit 1
    fi

    success "✓ Downloaded binary archive"

    # Download checksums
    info "Downloading checksums..."
    if ! download_file "$CHECKSUMS_URL" "checksums.txt"; then
        error "Error: Failed to download checksums"
        exit 1
    fi

    success "✓ Downloaded checksums"

    # Extract expected checksum
    EXPECTED_CHECKSUM="$(grep "$ARCHIVE_NAME" checksums.txt | awk '{print $1}')"
    if [ -z "$EXPECTED_CHECKSUM" ]; then
        error "Error: Checksum not found for $ARCHIVE_NAME"
        exit 1
    fi

    # Verify checksum
    info "Verifying checksum..."
    verify_checksum "$ARCHIVE_NAME" "$EXPECTED_CHECKSUM"

    # Extract archive
    info "Extracting archive..."
    tar -xzf "$ARCHIVE_NAME"

    if [ ! -f "$BINARY_NAME" ]; then
        error "Error: Binary not found in archive"
        exit 1
    fi

    # Make executable
    chmod +x "$BINARY_NAME"

    # Verify binary works
    info "Verifying binary..."
    if ! ./"$BINARY_NAME" --version >/dev/null 2>&1; then
        error "Error: Downloaded binary failed verification"
        exit 1
    fi

    NEW_VERSION="$(./"$BINARY_NAME" --version 2>/dev/null | head -n 1 || echo "unknown")"
    success "✓ Binary verified: $NEW_VERSION"
}

# Install the binary
install_binary() {
    # Create installation directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        if [ "$NEEDS_SUDO" = "1" ]; then
            sudo mkdir -p "$INSTALL_DIR"
        else
            mkdir -p "$INSTALL_DIR"
        fi
    fi

    TARGET_PATH="$INSTALL_DIR/$BINARY_NAME"

    info ""
    info "Installing to: $TARGET_PATH"

    # Install binary
    if [ "$NEEDS_SUDO" = "1" ]; then
        if ! sudo mv -f "$TEMP_DIR/$BINARY_NAME" "$TARGET_PATH"; then
            error "Error: Failed to install binary"
            exit 1
        fi
    else
        if ! mv -f "$TEMP_DIR/$BINARY_NAME" "$TARGET_PATH"; then
            error "Error: Failed to install binary"
            exit 1
        fi
    fi

    success "✓ Binary installed successfully"
}

# Detect user's shell
detect_shell() {
    case "$SHELL" in
        */zsh)
            SHELL_NAME="zsh"
            PROFILE_FILE="$HOME/.zshrc"
            ;;
        */bash)
            SHELL_NAME="bash"
            if [ -f "$HOME/.bashrc" ]; then
                PROFILE_FILE="$HOME/.bashrc"
            else
                PROFILE_FILE="$HOME/.bash_profile"
            fi
            ;;
        */fish)
            SHELL_NAME="fish"
            PROFILE_FILE="$HOME/.config/fish/config.fish"
            ;;
        *)
            SHELL_NAME="unknown"
            PROFILE_FILE="$HOME/.profile"
            ;;
    esac
}

# Update PATH if needed
update_path() {
    # Skip if PATH update not needed
    if [ "${NEEDS_PATH_UPDATE:-0}" != "1" ]; then
        return 0
    fi

    # Skip if MCP_K6_NO_PATH_UPDATE is set
    if [ "${MCP_K6_NO_PATH_UPDATE:-0}" = "1" ]; then
        info ""
        warning "Skipping PATH update (MCP_K6_NO_PATH_UPDATE is set)"
        warning "Add the following to your shell profile:"
        warning "  export PATH=\"$INSTALL_DIR:\$PATH\""
        return 0
    fi

    # Detect shell
    detect_shell

    info ""
    info "$BINARY_NAME is installed in: $INSTALL_DIR"
    info "This directory is not in your PATH."
    info ""
    info "The following line will be added to $PROFILE_FILE:"
    info "  export PATH=\"$INSTALL_DIR:\$PATH\""
    info ""

    # Prompt user in interactive mode
    if [ -t 0 ]; then
        printf "Add to PATH? [Y/n] "
        read -r response
        case "$response" in
            [nN]*)
                warning "Skipped PATH update."
                warning "To use $BINARY_NAME, add it to your PATH manually:"
                warning "  export PATH=\"$INSTALL_DIR:\$PATH\""
                return 0
                ;;
        esac
    fi

    # Add to PATH
    PATH_EXPORT="export PATH=\"$INSTALL_DIR:\$PATH\""

    # Create profile file if it doesn't exist
    if [ ! -f "$PROFILE_FILE" ]; then
        mkdir -p "$(dirname "$PROFILE_FILE")"
        touch "$PROFILE_FILE"
    fi

    # Check if already in profile
    if grep -q "$INSTALL_DIR" "$PROFILE_FILE" 2>/dev/null; then
        info "PATH already configured in $PROFILE_FILE"
    else
        printf "\n# Added by mcp-k6 installer\n%s\n" "$PATH_EXPORT" >> "$PROFILE_FILE"
        success "✓ Added to $PROFILE_FILE"
        info ""
        info "To use $BINARY_NAME in this session, run:"
        info "  export PATH=\"$INSTALL_DIR:\$PATH\""
        info ""
        info "Or start a new terminal session."
    fi
}

# Main installation function
main() {
    info "mcp-k6 installer"
    info "================"
    info ""

    # Check dependencies
    check_dependencies

    # Detect platform
    detect_platform

    # Determine installation directory
    determine_install_dir

    # Check sudo if needed
    if [ "${NEEDS_SUDO:-0}" = "1" ]; then
        check_sudo
    fi

    # Check for existing installation
    check_existing_installation

    # Get latest release tag
    get_latest_release_tag

    # Download and verify
    download_and_verify

    # Install binary
    install_binary

    # Update PATH if needed
    update_path

    # Final success message
    info ""
    success "═══════════════════════════════════════════════"
    success "✓ Installation completed successfully!"
    success "═══════════════════════════════════════════════"
    info ""
    info "Installed: $NEW_VERSION"
    info "Location: $INSTALL_DIR/$BINARY_NAME"
    info ""
    info "Verify installation:"
    info "  $BINARY_NAME --version"
    info ""
    info "To uninstall, run:"
    info "  curl -fsSL https://raw.githubusercontent.com/$REPO/main/uninstall.sh | sh"
    info ""
}

# Execute main function (wrapped to prevent partial execution from curl | sh)
main "$@"
