#!/bin/bash
# Download sherpa-onnx keyword spotting models for wake word detection

set -e

# Model download URL (English model)
KWS_MODEL_URL="https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models/sherpa-onnx-kws-zipformer-zh-en-3M-2025-12-20.tar.bz2"

# Get models directory from config or use default
MODELS_DIR="${MODELS_DIR:-$HOME/.local/share/tmux-stt/sherpa-kws}"

# Create models directory if it doesn't exist
mkdir -p "$MODELS_DIR"

# Check if models already exist
if [ -f "$MODELS_DIR/encoder-epoch-13-avg-2-chunk-16-left-64.onnx" ]; then
    echo "KWS models already exist in: $MODELS_DIR"
    echo "Skipping download. Remove the directory to re-download."
    exit 0
fi

echo "Downloading sherpa-onnx KWS models..."
echo "URL: $KWS_MODEL_URL"
echo "Destination: $MODELS_DIR"

# Create temporary directory for download
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Download the model archive
echo "Downloading model archive..."
curl -L -o kws-models.tar.bz2 "$KWS_MODEL_URL"

# Extract the archive
echo "Extracting models..."
tar xf kws-models.tar.bz2

# Move files to target directory
MODEL_SUBDIR=$(ls -d sherpa-onnx-kws-* 2>/dev/null | head -1)
if [ -z "$MODEL_SUBDIR" ]; then
    echo "ERROR: Could not find extracted model directory"
    exit 1
fi

echo "Moving files to: $MODELS_DIR"
mv "$MODEL_SUBDIR"/* "$MODELS_DIR/"

# Cleanup
cd /
rm -rf "$TEMP_DIR"

# Verify download was successful
if [ ! -f "$MODELS_DIR/encoder-epoch-13-avg-2-chunk-16-left-64.onnx" ]; then
    echo "ERROR: Failed to download KWS models"
    exit 1
fi

echo "Successfully downloaded sherpa-onnx KWS models"
echo "Model directory: $MODELS_DIR"

# List downloaded files
echo ""
echo "Downloaded files:"
ls -lh "$MODELS_DIR/"

echo ""
echo "To use sherpa-onnx KWS for wake word detection:"
echo "1. Create a keywords.txt file in $MODELS_DIR"
echo "2. Set wake word method to sherpa-kws"
echo "3. Example keywords.txt format:"
echo "   hey @hey"
echo "   computer @computer"
