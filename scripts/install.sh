#!/bin/sh
set -e

# DeploySentry CLI Install Script
# Downloads and installs the deploysentry binary from GitHub releases

REPO="shadsorg/DeploySentry"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="deploysentry"

# Release lag sentinel: binaries at or below this version predate the
# 2026-04-23 auth-flow fix. Incoming releases must bump this guard or
# remove it entirely.
STALE_VERSION_MAX="0.1.0"

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
    cat <<EOF
Need elevated permissions to install to $INSTALL_DIR.

To skip the sudo prompt, re-run with a user-writable directory:

  INSTALL_DIR=\$HOME/bin  sh -c "\$(curl -fsSL https://api.dr-sentry.com/install.sh)"

(Make sure \$HOME/bin is on your PATH.)
EOF
    SUDO="sudo"
  else
    echo "Error: $INSTALL_DIR is not writable and sudo is not available." >&2
    echo "  Try: INSTALL_DIR=\$HOME/bin sh -c \"\$(curl -fsSL https://api.dr-sentry.com/install.sh)\"" >&2
    exit 1
  fi
fi

$SUDO mv "$TEMP_DIR/$BINARY_FILE" "$INSTALL_DIR/$BINARY_NAME"
$SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo "Successfully installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"

# Verify installation and print version
VERSION_OUTPUT=""
if "$INSTALL_DIR/$BINARY_NAME" --version >/dev/null 2>&1; then
  echo "Version info:"
  VERSION_OUTPUT=$("$INSTALL_DIR/$BINARY_NAME" --version 2>&1)
  echo "$VERSION_OUTPUT"
else
  echo "Warning: Could not verify installation (--version flag not available)"
fi

# Warn loudly when the downloaded binary predates the auth-flow fix.
# The canonical `deploysentry auth login --token …` path did not exist
# in the v0.1.0 release; users running `auth login` with no flag get
# routed to a phantom OAuth endpoint.
case "$VERSION_OUTPUT" in
  *"version $STALE_VERSION_MAX"*|*"version 0.0."*)
    cat >&2 <<EOF

!! WARNING !!
The binary just installed ($VERSION_OUTPUT) predates the 2026-04-23
CLI auth-flow fix. 'deploysentry auth login --token ...' will fail
with 'unknown flag: --token'.

Two workarounds:
  1. Build from source:
       go install github.com/deploysentry/deploysentry/cmd/cli@main

  2. Skip 'auth login' on this binary and export the API key:
       export DEPLOYSENTRY_API_KEY=ds_live_...
     Every CLI command + the MCP server read that env var as a
     fallback when no credentials file exists.

See docs/Getting_Started.md §2 Troubleshooting for more.
EOF
    ;;
esac

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
