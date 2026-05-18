#!/bin/bash
# Download Silero VAD model for sherpa-onnx integration

set -e

# Model download URL
SILERO_VAD_URL="https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx"

# Get models directory from config or use default
MODELS_DIR="${MODELS_DIR:-$HOME/.local/share/tmux-stt/models}"

# Create models directory if it doesn't exist
mkdir -p "$MODELS_DIR"

# Model file path
MODEL_PATH="$MODELS_DIR/silero_vad.onnx"

# Check if model already exists
if [ -f "$MODEL_PATH" ]; then
    echo "Silero VAD model already exists at: $MODEL_PATH"
    echo "Skipping download. Remove the file to re-download."
    exit 0
fi

echo "Downloading Silero VAD model..."
echo "URL: $SILERO_VAD_URL"
echo "Destination: $MODEL_PATH"

# Download the model
curl -L -o "$MODEL_PATH" "$SILERO_VAD_URL"

# Verify download was successful
if [ ! -f "$MODEL_PATH" ]; then
    echo "ERROR: Failed to download Silero VAD model"
    exit 1
fi

# Check file size (should be around 629KB)
FILE_SIZE=$(stat -f%z "$MODEL_PATH" 2>/dev/null || stat -c%s "$MODEL_PATH" 2>/dev/null)
if [ "$FILE_SIZE" -lt 500000 ]; then
    echo "WARNING: Downloaded file size ($FILE_SIZE bytes) seems too small"
    echo "Expected size: ~629KB"
fi

echo "Successfully downloaded Silero VAD model"
echo "Model path: $MODEL_PATH"
echo "File size: $FILE_SIZE bytes"

echo ""
echo "To use Silero VAD, set the following configuration:"
echo "tmux-stt config --set vad.method=silero"
