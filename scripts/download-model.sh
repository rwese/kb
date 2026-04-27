#!/bin/bash
# Download GGUF embedding model
# Uses HuggingFace as source (requires network)
#
# Usage:
#   ./scripts/download-model.sh                    # Download to default ~/.cache/kb
#   ./scripts/download-model.sh /custom/path      # Download to custom directory

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUT_DIR="${1:-$HOME/.cache/kb}"

MODEL_NAME="all-MiniLM-L6-v2-Q4_K_M"
MODEL_FILE="${MODEL_NAME}.gguf"
REPO="second-state/All-MiniLM-L6-v2-Embedding-GGUF"
HF_URL="https://huggingface.co/$REPO/resolve/main/$MODEL_FILE"

echo "Downloading embedding model: $MODEL_NAME"
echo "Output directory: $OUT_DIR"
echo ""

mkdir -p "$OUT_DIR"

# Check if already downloaded
if [[ -f "$OUT_DIR/$MODEL_FILE" ]]; then
    echo "Model already exists at $OUT_DIR/$MODEL_FILE"
    echo "To re-download, delete the file first:"
    echo "  rm $OUT_DIR/$MODEL_FILE"
    exit 0
fi

# Download with progress
echo "Downloading from HuggingFace..."
curl -L "$HF_URL" \
    -o "$OUT_DIR/$MODEL_FILE" \
    --progress-bar \
    --location

# Verify download
if [[ -f "$OUT_DIR/$MODEL_FILE" ]]; then
    size=$(du -h "$OUT_DIR/$MODEL_FILE" | cut -f1)
    echo ""
    echo "Download complete!"
    echo "Model: $OUT_DIR/$MODEL_FILE"
    echo "Size: $size"
else
    echo "Download failed!" >&2
    exit 1
fi
