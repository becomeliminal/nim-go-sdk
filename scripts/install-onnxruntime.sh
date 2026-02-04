#!/bin/bash
set -e

# Install ONNX Runtime for Linux
# This script downloads and installs the ONNX Runtime C++ library

ONNX_VERSION="1.23.2"
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

echo "Installing ONNX Runtime ${ONNX_VERSION} for ${OS}-${ARCH}..."

# Determine the correct download URL based on architecture
if [ "$ARCH" = "x86_64" ]; then
    DOWNLOAD_NAME="onnxruntime-linux-x64-${ONNX_VERSION}"
elif [ "$ARCH" = "aarch64" ]; then
    DOWNLOAD_NAME="onnxruntime-linux-aarch64-${ONNX_VERSION}"
else
    echo "❌ Unsupported architecture: ${ARCH}"
    exit 1
fi

DOWNLOAD_URL="https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/${DOWNLOAD_NAME}.tgz"
INSTALL_DIR="$HOME/.local/lib/onnxruntime"

echo "Downloading from: ${DOWNLOAD_URL}"

# Create temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Download and extract
curl -L -o onnxruntime.tgz "$DOWNLOAD_URL"
tar -xzf onnxruntime.tgz

# Create installation directory
mkdir -p "$INSTALL_DIR"
mkdir -p "$HOME/.local/include"

# Copy files
echo "Installing to ${INSTALL_DIR}..."
cp -r "${DOWNLOAD_NAME}/lib/"* "$INSTALL_DIR/"
cp -r "${DOWNLOAD_NAME}/include/"* "$HOME/.local/include/"

# Add to ld.so.conf.d if we have sudo, otherwise use LD_LIBRARY_PATH
if [ -w /etc/ld.so.conf.d ]; then
    echo "$INSTALL_DIR" | sudo tee /etc/ld.so.conf.d/onnxruntime.conf
    sudo ldconfig
    echo "✅ Added to system library path"
else
    echo "⚠️  No sudo access. You'll need to set LD_LIBRARY_PATH:"
    echo "    export LD_LIBRARY_PATH=${INSTALL_DIR}:\$LD_LIBRARY_PATH"
    echo ""
    echo "Add this to your ~/.bashrc or ~/.zshrc to make it permanent"
fi

# Cleanup
cd -
rm -rf "$TMP_DIR"

echo ""
echo "✅ ONNX Runtime installed successfully!"
echo "   Library path: ${INSTALL_DIR}"
echo "   Headers path: ${HOME}/.local/include"
echo ""
echo "To verify installation:"
echo "   ldconfig -p | grep onnxruntime"
echo ""
echo "If you see errors, run:"
echo "   export LD_LIBRARY_PATH=${INSTALL_DIR}:\$LD_LIBRARY_PATH"
