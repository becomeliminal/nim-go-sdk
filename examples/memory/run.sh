#!/bin/bash
set -e

echo "================================================"
echo "  Nim Go SDK - Memory System Example"
echo "================================================"
echo ""

# Check if ANTHROPIC_API_KEY is set
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "❌ Error: ANTHROPIC_API_KEY environment variable is not set"
    echo ""
    echo "Please set it:"
    echo "  export ANTHROPIC_API_KEY='your-key-here'"
    echo ""
    exit 1
fi

# Check if ONNX model exists
if [ ! -f "../../models/all-MiniLM-L6-v2/model.onnx" ]; then
    echo "❌ Error: ONNX model not found"
    echo ""
    echo "Please download it:"
    echo "  cd ../.. && ./scripts/download-model.sh"
    echo ""
    exit 1
fi

# Check if ONNX Runtime is available
if [ ! -f "/home/jack/.local/lib/onnxruntime/libonnxruntime.so" ]; then
    echo "⚠️  Warning: ONNX Runtime not found at expected location"
    echo "   If build fails, install it:"
    echo "     cd ../.. && ./scripts/download-onnxruntime.sh"
    echo ""
fi

echo "✅ Prerequisites checked"
echo ""
echo "Building example (with ONNX tag)..."
go build -tags onnx -o memory-example

echo "✅ Build complete"
echo ""
echo "Running memory example..."
echo "================================================"
echo ""

./memory-example
