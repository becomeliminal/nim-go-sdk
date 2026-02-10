#!/bin/bash
set -e

cd "$(dirname "$0")"

if [ ! -f .env ]; then
    echo "No .env file found. Copy .env.example and fill in your keys:"
    echo "  cp .env.example .env"
    exit 1
fi

echo "Building yield optimizer..."
go build -o yield-optimizer .

echo "Starting server on :8080..."
./yield-optimizer
