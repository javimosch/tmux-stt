# tmux-stt Skill

Voice wake word + speech-to-text + tmux paste daemon for Linux.

## Overview

A background daemon that listens for a configurable wake word, transcribes voice input using local STT (Whisper.cpp), optionally translates text, and pastes the transcription into the active tmux pane.

## Key Features

- **Fully local STT**: Uses Whisper.cpp (tiny/base/small/medium models) with local caching
- **Wake word detection**: VAD + STT-based wake word detection (no custom models needed)
- **Configurable**: Wake word, model size, translation, audio settings, VAD parameters
- **Tmux integration**: Detects active pane and pastes text
- **Optional translation**: OpenAI-compatible API for language translation
- **Agent-friendly CLI**: JSON output, semantic exit codes, machine-readable help
- **Optimized capture**: Voice Activity Detection (VAD) with configurable parameters

## Project Structure

```
tmux-stt/
├── main.go              # CLI entry point and command handlers
├── daemon.go            # Daemon process management
├── internal/
│   ├── audio/           # PortAudio audio capture
│   ├── config/          # Configuration management
│   ├── server/          # Main server logic
│   ├── stt/             # Whisper.cpp wrapper
│   ├── tmux/            # Tmux integration
│   ├── translate/       # Translation service
│   └── wakeword/        # Wake word detection (VAD)
├── cmd/
│   └── test-audio/      # Audio testing tool
├── scripts/             # Installation scripts
└── docs/                # Documentation
```

## Quick Start

### Build
```bash
cd ~/ai/tmux-stt
chmod +x build.sh
./build.sh
```

### Basic Usage
```bash
# Start daemon in foreground (for testing)
tmux-stt start

# Start daemon in background
tmux-stt start -daemon

# Check status
tmux-stt status

# Stop daemon
tmux-stt stop
```

## Configuration

### View Configuration
```bash
tmux-stt config --list
tmux-stt --json config --list  # JSON format
```

### Set Configuration
```bash
# Wake word
tmux-stt config --set wake-word=TOTO

# STT language
tmux-stt config --set stt.language=es

# STT model size
tmux-stt config --set stt.model=base

# Strip wake word from output
tmux-stt config --set strip-wake-word=true

# VAD parameters (voice capture sensitivity)
tmux-stt config --set vad.threshold=700
tmux stt config --set vad.silence-ms=1500
tmux-stt config --set vad.speech-ms=600
```

### Important Configuration Parameters

**STT Model Sizes:**
- `tiny` (~77MB): Fastest, lower accuracy
- `base` (~148MB): Balanced speed/accuracy
- `small` (~75MB): Better accuracy, slower
- `medium` (~1.5GB): Best accuracy, slowest

**VAD Parameters (Voice Capture):**
- `vad.threshold`: Speech sensitivity (300-1000, higher = less sensitive)
- `vad.silence-ms`: Wait time after silence (500-2000ms)
- `vad.speech-ms`: Minimum speech duration (200-1000ms)

**Trade-offs:**
- Higher threshold = less background noise, but may miss quiet speech
- Longer silence wait = more reliable, but slower response
- Longer min speech = filters noise, but may miss short commands

## Commands

### Main Commands
- `start` - Start voice recognition daemon
- `stop` - Stop daemon
- `status` - Check daemon status
- `config` - Manage configuration
- `test` - Test components
- `tmux-list` - List tmux sessions/windows/panes
- `pipe` - Output transcriptions to stdout (for piping)
- `version` - Show version information

### Global Flags
- `--json` - Output in JSON format
- `--help-json` - Show machine-readable help in JSON format

### Config Commands
- `--list` - List all configuration
- `--set key=value` - Set configuration
- `## get key` - Get configuration

### Test Commands
- `--transcribe-only` - Test transcription without wake word
- `--wake-word` - Test wake word detection only
- `--translation` - Test translation functionality
- `--tmux` - Test tmux integration

## Architecture

```
Microphone → PortAudio → VAD (Voice Activity Detection) → Whisper.cpp (Wake Word + STT) → Translation (optional) → Tmux Paste
```

## Tech Stack

- **Language**: Go
- **STT**: Whisper.cpp binary wrapper (no CGO required)
- **Wake Word**: VAD + STT-based (energy threshold + transcription)
- **Audio**: PortAudio via CGO
- **Translation**: OpenAI-compatible API (optional)

## Model Caching

Models are cached in `~/.local/share/tmux-stt/models/`:
- `ggml-tiny.bin`
- `ggml-base.bin`
- `ggl-small.bin`
- `ggml-medium.bin`

Models are only downloaded once and reused for subsequent runs.

## Dependencies

### System Dependencies
```bash
sudo apt install portaudio19-dev
```

### Go Dependencies
```bash
go install github.com/gordonklaus/portaudio@latest
```

### Whisper.cpp
Built locally or installed. The project includes build scripts for whisper.cpp.

## Agent-Friendly Features

### Semantic Exit Codes
- `0`: Success
- `80-89`: User errors (invalid arguments, etc.)
- `90-99`: Resource errors (config issues, etc.)
- `100-109`: External errors (network, etc.)
- `110-119`: Internal errors

### JSON Output
All commands support `--json` flag for machine-readable output.

### Machine-Readable Help
Use `--help-json` for structured help documentation.

## Use Cases

- **Voice commands in tmux**: Execute terminal commands by voice
- **Hands-free coding**: Dictate code while keeping hands on keyboard
- **Accessibility**: Voice control for terminal operations
- **Multilingual**: Supports 99 languages via Whisper models
- **Custom workflows**: Pipe transcription to other tools

## Common Issues

### Voice Capture Not Working
1. Check audio device: `tmux-stt config --get audio.device`
2. Adjust VAD parameters: `tmux-stt config --set vad.threshold=500`
3. Test audio capture: `tmux-stt test --transcribe-only`

### Wake Word Not Detected
1. Try simpler wake word: `tmux-stt config --set wake-word=hey`
2. Check STT language matches your speech
3. Test wake word: `tmux-stt test --wake-word`

### Model Not Working
1. Check model is cached: `ls ~/.local/share/turmux-stt/models/`
2. Try different model size: `tmux-stt config --set stt.model=base`
3. Re-download model if corrupted: `rm ~/.local/share/tmux-stt/models/ggml-small.bin`

## Current Configuration (Default)

- Wake word: "TOTO"
- STT Language: Spanish (es)
- STT Model: base
- Strip wake word: true
- VAD Threshold: 700
- VAD Silence: 1500ms
- VAD Min speech: 600ms

## Example Usage

```bash
# Start daemon
tmux-stt start -daemon

# In a tmux session, say:
# "TOTO lista archivos"
# Result: "lista archivos" (pasted to terminal, wake word removed)

# "TOTO crea un directorio"
# Result: "crea un directorio" (pasted to terminal)

# "TOTO ¿qué hora es?"
# Result: "¿qué hora es?" (pasted to terminal)
```

## Development Status

- [x] Project structure and CLI interface
- [x] Configuration system
- [x] Audio input (PortAudio integration)
- [x] Wake word detection (VAD + STT-based)
- [x] STT integration (Whisper.cpp binary wrapper)
- [x] Translation service (OpenAI-compatible API)
- [x] Tmux integration (paste to active pane)
- [x] Daemon process management
- [x] Model caching and download
- [x] Agent-friendly CLI (JSON output, semantic exit codes)
- [x] Voice Activity Detection (VAD) optimization
- [x] Wake word stripping from output
- [ ] Web UI (configuration interface) - Optional future enhancement

## License

MIT
