#!/bin/bash
set -e

# Download script for all-MiniLM-L6-v2 embedding model
# This model is used for local semantic search in the memory system

echo "Creating models directory..."
mkdir -p models/all-MiniLM-L6-v2

cd models/all-MiniLM-L6-v2

echo "Downloading all-MiniLM-L6-v2 ONNX model (~80MB)..."
if [ ! -f model.onnx ]; then
    curl -L -o model.onnx https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx
    echo "✓ Model downloaded"
else
    echo "✓ Model already exists"
fi

echo "Downloading tokenizer..."
if [ ! -f tokenizer.json ]; then
    curl -L -o tokenizer.json https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json
    echo "✓ Tokenizer downloaded"
else
    echo "✓ Tokenizer already exists"
fi

echo ""
echo "✓ Setup complete!"
echo ""
echo "Model files are in: $(pwd)"
echo ""
echo "Usage in code:"
echo "  embedder, err := onnx.New(onnx.Config{"
echo "    ModelPath:     \"models/all-MiniLM-L6-v2/model.onnx\","
echo "    TokenizerPath: \"models/all-MiniLM-L6-v2/tokenizer.json\","
echo "    Dimensions:    384,"
echo "  })"
