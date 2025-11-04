#!/bin/sh
#
# mcp-k6 uninstaller
#
# This script removes the mcp-k6 MCP server binary and optionally cleans up
# configuration files and PATH modifications.
#
# Usage:
#   sh uninstall.sh
#   curl -fsSL https://raw.githubusercontent.com/grafana/mcp-k6/main/uninstall.sh | sh
#

set -e
set -u

BINARY_NAME="mcp-k6"
POSSIBLE_LOCATIONS="
/usr/local/bin
$HOME/.local/bin
$HOME/bin
$HOME/.mcp-k6/bin
"

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

# Find all installations
find_installations() {
    FOUND_LOCATIONS=""

    for location in $POSSIBLE_LOCATIONS; do
        if [ -f "$location/$BINARY_NAME" ]; then
            FOUND_LOCATIONS="$FOUND_LOCATIONS$location/$BINARY_NAME
"
        fi
    done

    if [ -z "$FOUND_LOCATIONS" ]; then
        return 1
    fi

    return 0
}

# Remove binary with sudo handling
remove_binary() {
    local binary_path="$1"
    local dir_path
    dir_path="$(dirname "$binary_path")"

    info "Removing: $binary_path"

    # Check if we need sudo
    if [ -w "$binary_path" ]; then
        # We have write permission, no sudo needed
        rm -f "$binary_path"
        success "✓ Removed $binary_path"
    else
        # Need sudo for removal
        if ! command_exists sudo; then
            error "Error: sudo is required to remove $binary_path but not found"
            return 1
        fi

        # Check for passwordless sudo
        if sudo -n -v >/dev/null 2>&1; then
            sudo rm -f "$binary_path"
            success "✓ Removed $binary_path"
        elif [ -t 0 ]; then
            # Interactive mode
            warning "This will run 'sudo rm $binary_path'"
            printf "Please enter your password when prompted.\n"
            sudo rm -f "$binary_path"
            success "✓ Removed $binary_path"
        else
            # Non-interactive and no passwordless sudo
            error "Error: Cannot remove $binary_path (no sudo access)"
            return 1
        fi
    fi

    return 0
}

# Remove installation directory if empty
remove_directory_if_empty() {
    local dir_path="$1"

    # Only remove ~/.mcp-k6 directory, not standard bin directories
    if [ "$dir_path" != "$HOME/.mcp-k6/bin" ]; then
        return 0
    fi

    local parent_dir="$HOME/.mcp-k6"

    if [ ! -d "$parent_dir" ]; then
        return 0
    fi

    # Check if directory is empty (only contains bin subdir which might be empty)
    if [ -z "$(find "$parent_dir" -mindepth 1 -type f)" ]; then
        if [ -t 0 ]; then
            info ""
            info "Directory $parent_dir is now empty."
            printf "Remove it? [Y/n] "
            read -r response
            case "$response" in
                [nN]*)
                    info "Kept $parent_dir"
                    return 0
                    ;;
            esac
        fi

        rm -rf "$parent_dir"
        success "✓ Removed $parent_dir"
    fi
}

# Detect user's shell profiles
detect_shell_profiles() {
    PROFILE_FILES=""

    if [ -f "$HOME/.bashrc" ]; then
        PROFILE_FILES="$PROFILE_FILES$HOME/.bashrc
"
    fi

    if [ -f "$HOME/.bash_profile" ]; then
        PROFILE_FILES="$PROFILE_FILES$HOME/.bash_profile
"
    fi

    if [ -f "$HOME/.zshrc" ]; then
        PROFILE_FILES="$PROFILE_FILES$HOME/.zshrc
"
    fi

    if [ -f "$HOME/.profile" ]; then
        PROFILE_FILES="$PROFILE_FILES$HOME/.profile
"
    fi

    if [ -f "$HOME/.config/fish/config.fish" ]; then
        PROFILE_FILES="$PROFILE_FILES$HOME/.config/fish/config.fish
"
    fi
}

# Remove PATH entries from shell profiles
remove_path_entries() {
    detect_shell_profiles

    if [ -z "$PROFILE_FILES" ]; then
        return 0
    fi

    # Check if any profile contains mcp-k6 PATH entries
    FOUND_IN_PROFILES=""
    for profile in $PROFILE_FILES; do
        if grep -q "mcp-k6" "$profile" 2>/dev/null; then
            FOUND_IN_PROFILES="$FOUND_IN_PROFILES$profile
"
        fi
    done

    if [ -z "$FOUND_IN_PROFILES" ]; then
        return 0
    fi

    info ""
    info "Found mcp-k6 PATH entries in shell profiles:"
    printf "%s" "$FOUND_IN_PROFILES"

    if [ -t 0 ]; then
        info ""
        printf "Remove PATH entries from shell profiles? [y/N] "
        read -r response
        case "$response" in
            [yY]*)
                ;;
            *)
                info "Kept PATH entries in shell profiles"
                warning "You may want to manually remove them."
                return 0
                ;;
        esac
    else
        # Non-interactive mode, skip removal
        warning "Skipping PATH removal (non-interactive mode)"
        warning "You may want to manually remove PATH entries from:"
        printf "%s" "$FOUND_IN_PROFILES"
        return 0
    fi

    # Remove lines containing mcp-k6
    for profile in $FOUND_IN_PROFILES; do
        # Create backup
        cp "$profile" "${profile}.bak"

        # Remove lines with "mcp-k6" and the comment line before it if it's the installer comment
        sed -i.tmp '/# Added by mcp-k6 installer/d; /mcp-k6/d' "$profile"
        rm -f "${profile}.tmp"

        success "✓ Removed PATH entries from $profile"
        info "  Backup saved as: ${profile}.bak"
    done
}

# Main uninstallation function
main() {
    info "mcp-k6 uninstaller"
    info "=================="
    info ""

    # Find all installations
    if ! find_installations; then
        warning "No installations of $BINARY_NAME found."
        info ""
        info "Checked locations:"
        printf "%s\n" "$POSSIBLE_LOCATIONS" | sed 's/^/  /'
        exit 0
    fi

    info "Found installations:"
    printf "%s" "$FOUND_LOCATIONS" | sed 's/^/  /'
    info ""

    # Remove each installation
    REMOVED_ANY=0
    for binary in $FOUND_LOCATIONS; do
        if remove_binary "$binary"; then
            REMOVED_ANY=1

            # Check if we should remove the installation directory
            dir_path="$(dirname "$binary")"
            remove_directory_if_empty "$dir_path"
        fi
    done

    if [ "$REMOVED_ANY" = "0" ]; then
        error "Failed to remove any installations"
        exit 1
    fi

    # Offer to remove PATH entries
    remove_path_entries

    # Final success message
    info ""
    success "═══════════════════════════════════════════════"
    success "✓ Uninstallation completed successfully!"
    success "═══════════════════════════════════════════════"
    info ""
    info "To reinstall, run:"
    info "  curl -fsSL https://raw.githubusercontent.com/grafana/mcp-k6/main/install.sh | sh"
    info ""
}

# Execute main function (wrapped to prevent partial execution from curl | sh)
main "$@"
