package stt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type WhisperModel struct {
	Name     string
	Size     string
	Path     string
	Download string
}

type TranscriptionResult struct {
	Text      string
	Language  string
	Duration  float64
	Segments []Segment
}

type Segment struct {
	Start float64
	End   float64
	Text  string
}

const (
	ModelTiny   = "tiny"
	ModelBase   = "base"
	ModelSmall  = "small"
	ModelMedium = "medium"
)

var ModelDownloads = map[string]string{
	ModelTiny:   "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
	ModelBase:   "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
	ModelSmall:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
	ModelMedium: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
}

type WhisperEngine struct {
	modelPath string
	modelSize  string
}

func NewWhisperEngine(modelsDir, modelSize string) (*WhisperEngine, error) {
	// Create models directory if it doesn't exist
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create models directory: %w", err)
	}

	// Validate model size
	if _, ok := ModelDownloads[modelSize]; !ok {
		return nil, fmt.Errorf("invalid model size: %s (valid: tiny, base, small, medium)", modelSize)
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))

	return &WhisperEngine{
		modelPath: modelPath,
		modelSize: modelSize,
	}, nil
}

func (w *WhisperEngine) EnsureModel() error {
	// Check if model already exists
	if _, err := os.Stat(w.modelPath); err == nil {
		fmt.Printf("Model already exists: %s\n", w.modelPath)
		return nil
	}

	// Download model
	downloadURL := ModelDownloads[w.modelSize]
	fmt.Printf("Downloading model from %s...\n", downloadURL)

	// Use curl to download
	cmd := exec.Command("curl", "-L", "-o", w.modelPath, downloadURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}

	fmt.Printf("Model downloaded successfully: %s\n", w.modelPath)
	return nil
}

func (w *WhisperEngine) Transcribe(audioFile string) (*TranscriptionResult, error) {
	// For now, this is a placeholder that would call whisper.cpp
	// In a full implementation, this would use CGO bindings to whisper.cpp
	
	// Placeholder: Simulate transcription
	return &TranscriptionResult{
		Text:     "This is a placeholder transcription result",
		Language: "en",
		Duration: 2.0,
	}, nil
}

func (w *WhisperEngine) TranscribeRaw(audioData []int16, sampleRate int) (*TranscriptionResult, error) {
	// For now, save audio data to a temporary file and transcribe
	// In a full implementation, this would pass raw audio directly to whisper.cpp
	
	// Create temporary audio file
	tmpFile, err := os.CreateTemp("", "whisper-*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Convert raw audio to WAV format (placeholder)
	// In real implementation, this would use proper WAV encoding
	
	// Transcribe the file
	return w.Transcribe(tmpFile.Name())
}

func (w *WhisperEngine) GetModelInfo() WhisperModel {
	return WhisperModel{
		Name:     w.modelSize,
		Size:     w.modelSize,
		Path:     w.modelPath,
		Download: ModelDownloads[w.modelSize],
	}
}
