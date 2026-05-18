#!/bin/bash

set -e

echo "Installing whisper.cpp..."

# Install dependencies
sudo apt update
sudo apt install -y git build-essential cmake libopenblas-dev

# Clone whisper.cpp
cd /tmp
if [ -d "whisper.cpp" ]; then
    echo "whisper.cpp directory already exists, updating..."
    cd whisper.cpp
    git pull
else
    echo "Cloning whisper.cpp..."
    git clone https://github.com/ggerganov/whisper.cpp
    cd whisper.cpp
fi

# Build whisper.cpp
echo "Building whisper.cpp..."
mkdir -p build
cd build
cmake ..
make -j$(nproc)

# Install binary
sudo cp ./main /usr/local/bin/whisper.cpp
sudo chmod +x /usr/local/bin/whisper.cpp

echo "✅ whisper.cpp installed successfully to /usr/local/bin/whisper.cpp"
echo "You can now use it for speech-to-text transcription."
