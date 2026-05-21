#!/usr/bin/env bash

# Juggle installation script
# Downloads and installs the latest version of Juggle

set -e

# Configuration
REPO="jeprecated/juggle"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="juggle"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${GREEN}==>${NC} $1"
}

warn() {
    echo -e "${YELLOW}Warning:${NC} $1"
}

error() {
    echo -e "${RED}Error:${NC} $1" >&2
    exit 1
}

# Detect OS and architecture
detect_platform() {
    local os
    local arch

    # Detect OS
    case "$(uname -s)" in
        Darwin*)
            os="darwin"
            ;;
        Linux*)
            os="linux"
            ;;
        *)
            error "Unsupported operating system: $(uname -s)"
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        *)
            error "Unsupported architecture: $(uname -m)"
            ;;
    esac

    echo "${os}-${arch}"
}

# Get latest release version from GitHub
get_latest_version() {
    local version
    version=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
        error "Failed to fetch latest version"
    fi
    
    echo "$version"
}

# Download and install binary
install_binary() {
    local platform="$1"
    local version="$2"
    local tmp_dir
    local download_url
    # Convert platform from "linux-amd64" to "linux_amd64" and strip v prefix from version
    local version_num="${version#v}"
    local platform_underscore="${platform//-/_}"
    local archive_name="${BINARY_NAME}_${version_num}_${platform_underscore}.tar.gz"

    info "Installing Juggle ${version} for ${platform}..."

    # Create temporary directory
    tmp_dir=$(mktemp -d)
    trap "rm -rf ${tmp_dir}" EXIT

    # Construct download URL
    download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"

    info "Downloading from ${download_url}..."
    if ! curl -L -o "${tmp_dir}/${archive_name}" "${download_url}"; then
        error "Failed to download Juggle"
    fi

    # Extract archive
    info "Extracting archive..."
    if ! tar -xzf "${tmp_dir}/${archive_name}" -C "${tmp_dir}"; then
        error "Failed to extract archive"
    fi

    # Create install directory if it doesn't exist
    mkdir -p "${INSTALL_DIR}"

    # Move binary to install directory
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    info "Installation complete!"
}

# Check if install directory is in PATH
check_path() {
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        warn "${INSTALL_DIR} is not in your PATH"
        echo
        echo "Add the following line to your shell configuration file:"
        echo "  (e.g., ~/.bashrc, ~/.zshrc, ~/.profile)"
        echo
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo
        echo "Then reload your shell configuration:"
        echo "  source ~/.bashrc  # or source ~/.zshrc"
        echo
    fi
}

# Main installation flow
main() {
    echo "Juggle Installer"
    echo "================="
    echo

    # Check for required commands
    for cmd in curl tar; do
        if ! command -v "$cmd" &> /dev/null; then
            error "Required command not found: $cmd"
        fi
    done

    # Detect platform
    local platform
    platform=$(detect_platform)
    info "Detected platform: ${platform}"

    # Get latest version
    local version
    version=$(get_latest_version)
    info "Latest version: ${version}"

    # Install binary
    install_binary "${platform}" "${version}"

    # Check PATH
    check_path

    # Verify installation
    echo
    info "Verifying installation..."
    if "${INSTALL_DIR}/${BINARY_NAME}" --version &> /dev/null; then
        echo
        echo -e "${GREEN}✓${NC} Juggle installed successfully!"
        echo
        echo "Get started:"
        echo "  juggle \"your prompt here\" --dry-run"
        echo "  juggle @task.md -n 5"
        echo
        echo "For more information:"
        echo "  juggle --help"
        echo "  https://github.com/${REPO}"
    else
        error "Installation verification failed"
    fi
}

# Handle build from source fallback
build_from_source() {
    info "Pre-built binaries not available, attempting to build from source..."
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        error "Go is required to build from source. Install Go from https://golang.org/dl/"
    fi

    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap "rm -rf ${tmp_dir}" EXIT

    info "Cloning repository..."
    if ! git clone "https://github.com/${REPO}.git" "${tmp_dir}/juggle"; then
        error "Failed to clone repository"
    fi

    cd "${tmp_dir}/juggle"
    
    info "Building binary..."
    if ! go build -o "${BINARY_NAME}" ./cmd/juggle; then
        error "Failed to build binary"
    fi

    # Install binary
    mkdir -p "${INSTALL_DIR}"
    mv "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    info "Build and installation complete!"
}

# Run main installation, fall back to building from source if needed
if ! main; then
    warn "Standard installation failed"
    build_from_source
    check_path
fi
