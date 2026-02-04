#!/bin/bash
# Run hackathon-starter with ONNX Runtime library path

export LD_LIBRARY_PATH=/home/jack/.local/lib/onnxruntime:$LD_LIBRARY_PATH

echo "Starting hackathon-starter with ONNX Runtime..."
go run main.go
