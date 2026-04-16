#!/bin/sh
set -e

# DeploySentry CLI Install Script
# Downloads and installs the deploysentry binary from GitHub releases

REPO="shadsorg/DeploySentry"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="deploysentry"

# Detect OS
OS=$(uname -s)
case "$OS" in
  Linux)
    OS="linux"
    ;;
  Darwin)
    OS="darwin"
    ;;
  *)
    echo "Error: Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH=$(uname -m)
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
    echo "Error: Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

echo "Detected OS: $OS, Architecture: $ARCH"

# Construct download URL
BINARY_FILE="${BINARY_NAME}-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_FILE}"

echo "Downloading from: $DOWNLOAD_URL"

# Create temporary directory for download
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Download binary using curl or wget
if command -v curl >/dev/null 2>&1; then
  if ! curl -fsSL -o "$TEMP_DIR/$BINARY_FILE" "$DOWNLOAD_URL"; then
    echo "Error: Failed to download binary with curl" >&2
    exit 1
  fi
elif command -v wget >/dev/null 2>&1; then
  if ! wget -q -O "$TEMP_DIR/$BINARY_FILE" "$DOWNLOAD_URL"; then
    echo "Error: Failed to download binary with wget" >&2
    exit 1
  fi
else
  echo "Error: Neither curl nor wget found. Please install one of them." >&2
  exit 1
fi

# Verify download succeeded
if [ ! -f "$TEMP_DIR/$BINARY_FILE" ]; then
  echo "Error: Downloaded file not found" >&2
  exit 1
fi

# Move binary to install directory (use sudo if needed)
SUDO=""
if [ ! -w "$INSTALL_DIR" ]; then
  if command -v sudo >/dev/null 2>&1; then
    echo "Need elevated permissions to install to $INSTALL_DIR"
    SUDO="sudo"
  else
    echo "Error: $INSTALL_DIR is not writable and sudo is not available." >&2
    echo "  Try: INSTALL_DIR=\$HOME/.local/bin sh -c \"\$(curl -fsSL https://api.dr-sentry.com/install.sh)\"" >&2
    exit 1
  fi
fi

$SUDO mv "$TEMP_DIR/$BINARY_FILE" "$INSTALL_DIR/$BINARY_NAME"
$SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo "Successfully installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"

# Verify installation and print version
if "$INSTALL_DIR/$BINARY_NAME" --version >/dev/null 2>&1; then
  echo "Version info:"
  "$INSTALL_DIR/$BINARY_NAME" --version
else
  echo "Warning: Could not verify installation (--version flag not available)"
fi

# Auto-register MCP server if Claude Code is installed
if command -v claude >/dev/null 2>&1; then
  echo ""
  echo "Claude Code detected. Registering DeploySentry MCP server..."
  if claude mcp add deploysentry -- "$INSTALL_DIR/$BINARY_NAME" mcp serve 2>/dev/null; then
    echo "MCP server registered. Restart Claude Code to use DeploySentry tools."
    echo "  Tools: ds_status, ds_list_orgs, ds_list_projects, ds_list_apps,"
    echo "         ds_list_environments, ds_create_api_key, ds_generate_workflow,"
    echo "         ds_list_flags, ds_get_flag, ds_create_flag, ds_toggle_flag"
  else
    echo "Could not auto-register MCP server. You can add it manually:"
    echo "  claude mcp add deploysentry -- $INSTALL_DIR/$BINARY_NAME mcp serve"
  fi
fi
