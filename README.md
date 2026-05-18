# tmux-stt

Voice wake word + speech-to-text + tmux paste daemon for Linux.

## Overview

A background daemon that:
1. Listens for a configurable wake word (default: "Hey mate")
2. Transcribes voice input using local STT
3. Optionally translates text (via OpenAI-compatible API)
4. Pastes transcription into active tmux pane

## Features

- **Fully local STT**: Uses Whisper.cpp (tiny/base/small/medium models)
- **Wake word detection**: VAD + STT-based wake word detection (no custom models needed)
- **Configurable**: Wake word, model size, translation, audio settings
- **Tmux integration**: Detects active pane and pastes text
- **Optional translation**: OpenAI-compatible API for language translation
- **Low resource usage**: Optimized for minimal CPU/memory footprint

## Architecture

```
Microphone → PortAudio → VAD (Voice Activity Detection) → Whisper.cpp (Wake Word + STT) → Translation (optional) → Tmux Paste
```

## Tech Stack

- **Language**: Go (for daemon management, configuration, tmux integration)
- **STT**: Whisper.cpp binary wrapper (no CGO required)
- **Wake Word**: VAD + STT-based (energy threshold + transcription)
- **Audio**: PortAudio via CGO (C library)
- **Translation**: OpenAI-compatible API (optional)

## Installation

### Prerequisites

```bash
# System dependencies
sudo apt install portaudio19-dev

# Go dependencies
go install github.com/gordonklaus/portaudio@latest

# Whisper.cpp (built locally or installed)
# The project includes a build script for whisper.cpp
```

### Build

```bash
cd ~/ai/tmux-stt
chmod +x build.sh
./build.sh
```

## Usage

### Start daemon

```bash
# Start in foreground (for testing)
tmux-stt start

# Start in background (daemon mode)
tmux-stt start -daemon

# Custom wake word and model
tmux-stt start -daemon -wake-word "Computer" -model base
```

### Configuration

```bash
# List all configuration
tmux-stt config --list

# Set wake word
tmux-stt config --set wake-word="Hey Jarvis"

# Set STT model size
tmux-stt config --set model=base

# Enable translation
tmux-stt config --set translation.enabled=true
tmux-stt config --set translation.source=es
tmux-stt config --set translation.target=en
tmux-stt config --set translation.endpoint=http://localhost:11434/v1
```

### Testing

```bash
# Test transcription without wake word
tmux-stt test --transcribe-only

# Test wake word detection only
tmux-stt test --wake-word

# Test translation functionality
tmux-stt test --translation

# Test tmux integration (must be run inside tmux)
tmux-stt test --tmux
```

### Daemon management

```bash
# Check status
tmux-stt status

# Stop daemon
tmux-stt stop
```

## Configuration

### Default Configuration

```json
{
  "wake_word": "Hey mate",
  "model_size": "tiny",
  "translation": {
    "enabled": false,
    "source_lang": "auto",
    "target_lang": "en",
    "api_endpoint": "http://localhost:11434/v1",
    "api_key": ""
  },
  "tmux": {
    "socket_path": "",
    "auto_enter": true
  },
  "audio": {
    "input_device": "default",
    "sample_rate": 16000,
    "channels": 1,
    "chunk_size": 1024
  },
  "models_dir": "~/.local/share/tmux-stt/models"
}
```

### STT Model Sizes

- **tiny** (~40MB): Fastest, lower accuracy (default)
- **base** (~80MB): Balanced speed/accuracy
- **small** (~200MB): Better accuracy, slower
- **medium** (~500MB): Best accuracy, slowest

## Implementation Status

- [x] Project structure and CLI interface
- [x] Configuration system
- [x] Audio input (PortAudio integration)
- [x] Wake word detection (VAD + STT-based)
- [x] STT integration (Whisper.cpp binary wrapper)
- [x] Translation service (OpenAI-compatible API)
- [x] Tmux integration (paste to active pane)
- [x] Daemon process management
- [ ] Web UI (configuration interface) - Optional future enhancement

## Quick Start

1. Build the project:
```bash
cd ~/ai/tmux-stt
./build.sh
```

2. Start a tmux session (required for full functionality):
```bash
tmux new -s voice
```

3. Start the voice daemon:
```bash
./tmux-stt start
```

4. Say "Hey mate" followed by your command
5. The transcription will be pasted into your tmux pane

## Development Plan

### Phase 1: Core Audio + STT ✅
1. PortAudio integration for audio capture
2. Whisper.cpp binary wrapper
3. Basic transcription (no wake word)
4. Model download and caching

### Phase 2: Wake Word Detection ✅
1. VAD (Voice Activity Detection) implementation
2. STT-based wake word detection
3. Passive listening loop
4. Trigger STT on wake word

### Phase 3: Translation ✅
1. OpenAI-compatible API client
2. Translation pipeline
3. Error handling and fallback

### Phase 4: Tmux Integration ✅
1. Active pane detection
2. Text paste functionality
3. Configuration for tmux socket
4. Error handling for tmux not running

### Phase 5: Polish ✅
1. Daemon process management
2. Logging and monitoring
3. Performance optimization

### Future Enhancements
- Web UI for configuration
- Custom wake word models (Sherpa-onnx)
- Real-time streaming STT
- Multiple language support

## License

MIT

## Credits

- [Whisper.cpp](https://github.com/ggerganov/whisper.cpp) - STT engine
- [PortAudio](http://www.portaudio.com/) - Audio I/O
