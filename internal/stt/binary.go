package stt

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WhisperBinary wraps the whisper.cpp binary for speech-to-text
type WhisperBinary struct {
	binaryPath string
	modelPath  string
	modelSize  string
	language   string
}

type WhisperOutput struct {
	Text      string   `json:"text"`
	Segments []Segment `json:"segments,omitempty"`
	Language string   `json:"language,omitempty"`
}

// NewWhisperBinary creates a new whisper.cpp wrapper
func NewWhisperBinary(modelsDir, modelSize string) (*WhisperBinary, error) {
	return NewWhisperBinaryWithLanguage(modelsDir, modelSize, "auto")
}

// NewWhisperBinaryWithLanguage creates a new whisper.cpp wrapper with specific language
func NewWhisperBinaryWithLanguage(modelsDir, modelSize, language string) (*WhisperBinary, error) {
	// Check if whisper.cpp binary exists in /tmp first (local build)
	localBinary := "/tmp/whisper.cpp-local"
	binaryPath := "/usr/local/bin/whisper.cpp" // Default installation path
	
	// Try local binary first
	if _, err := os.Stat(localBinary); err == nil {
		binaryPath = localBinary
		fmt.Printf("Using local whisper.cpp: %s\n", binaryPath)
	} else if _, err := os.Stat(binaryPath); err == nil {
		// Use system installation
		fmt.Printf("Using system whisper.cpp: %s\n", binaryPath)
	} else {
		// Try to find it in PATH
		path, err := exec.LookPath("whisper.cpp")
		if err != nil {
			return nil, fmt.Errorf("whisper.cpp binary not found. Please install it from https://github.com/ggerganov/whisper.cpp")
		}
		binaryPath = path
		fmt.Printf("Using whisper.cpp from PATH: %s\n", binaryPath)
	}

	// Create models directory if it doesn't exist
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create models directory: %w", err)
	}

	// Validate model size
	if _, ok := ModelDownloads[modelSize]; !ok {
		return nil, fmt.Errorf("invalid model size: %s (valid: tiny, base, small, medium)", modelSize)
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))

	return &WhisperBinary{
		binaryPath: binaryPath,
		modelPath:  modelPath,
		modelSize:  modelSize,
		language:   language,
	}, nil
}

// EnsureModel downloads the model if it doesn't exist
func (w *WhisperBinary) EnsureModel() error {
	return w.EnsureModelQuiet(false)
}

// EnsureModelQuiet downloads the model if it doesn't exist, with optional quiet mode
func (w *WhisperBinary) EnsureModelQuiet(quiet bool) error {
	// Check if model already exists
	if _, err := os.Stat(w.modelPath); err == nil {
		if !quiet {
			fmt.Printf("Model already exists: %s\n", w.modelPath)
		}
		return nil
	}

	// Download model
	downloadURL := ModelDownloads[w.modelSize]
	if !quiet {
		fmt.Printf("Downloading %s model from %s...\n", w.modelSize, downloadURL)
	}

	// Use curl to download
	cmd := exec.Command("curl", "-L", "-o", w.modelPath, downloadURL)
	if !quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}

	if !quiet {
		fmt.Printf("Model downloaded successfully: %s\n", w.modelPath)
	}
	return nil
}

// TranscribeFile transcribes an audio file
func (w *WhisperBinary) TranscribeFile(audioFile string) (*TranscriptionResult, error) {
	return w.TranscribeFileQuiet(audioFile, false)
}

// TranscribeFileQuiet transcribes an audio file with optional quiet mode
func (w *WhisperBinary) TranscribeFileQuiet(audioFile string, quiet bool) (*TranscriptionResult, error) {
	if err := w.EnsureModelQuiet(quiet); err != nil {
		return nil, err
	}

	// Build whisper.cpp command
	args := []string{
		"-m", w.modelPath,
		"-f", audioFile,
		"-otxt",        // Output text only
		"-of", "/tmp/whisper-output", // Output file prefix
	}

	// Add language parameter if specified (not "auto")
	if w.language != "" && w.language != "auto" {
		args = append(args, "-l", w.language)
	}

	// Run whisper.cpp
	cmd := exec.Command(w.binaryPath, args...)
	
	if !quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Running: %s %s\n", w.binaryPath, strings.Join(args, " "))
	} else {
		// Suppress output in quiet mode
		cmd.Stdout = nil
		cmd.Stderr = nil
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("whisper.cpp failed: %w", err)
	}

	// Read the output file
	outputFile := "/tmp/whisper-output.txt"
	content, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file: %w", err)
	}

	// Clean up output file
	os.Remove(outputFile)

	text := strings.TrimSpace(string(content))
	if text == "" {
		return nil, fmt.Errorf("empty transcription result")
	}

	return &TranscriptionResult{
		Text:     text,
		Language: "auto",
	}, nil
}

// TranscribeFileWithJSON transcribes an audio file and returns structured output
func (w *WhisperBinary) TranscribeFileWithJSON(audioFile string) (*TranscriptionResult, error) {
	if err := w.EnsureModel(); err != nil {
		return nil, err
	}

	// Build whisper.cpp command with JSON output
	args := []string{
		"-m", w.modelPath,
		"-f", audioFile,
		"-oj",          // Output JSON
		"-of", "/tmp/whisper-output",
	}

	// Add language parameter if specified (not "auto")
	if w.language != "" && w.language != "auto" {
		args = append(args, "-l", w.language)
	}

	// Run whisper.cpp
	cmd := exec.Command(w.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("whisper.cpp failed: %w, output: %s", err, string(output))
	}

	// Parse JSON output
	var whisperOutput WhisperOutput
	if err := json.Unmarshal(output, &whisperOutput); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	// Convert to our format
	result := &TranscriptionResult{
		Text:     whisperOutput.Text,
		Language: whisperOutput.Language,
	}

	for _, seg := range whisperOutput.Segments {
		result.Segments = append(result.Segments, Segment{
			Start: seg.Start,
			End:   seg.End,
			Text:  seg.Text,
		})
	}

	return result, nil
}

// TranscribeRaw transcribes raw audio data
func (w *WhisperBinary) TranscribeRaw(audioData []int16, sampleRate int) (*TranscriptionResult, error) {
	return w.TranscribeRawQuiet(audioData, sampleRate, false)
}

// TranscribeRawQuiet transcribes raw audio data with optional quiet mode
func (w *WhisperBinary) TranscribeRawQuiet(audioData []int16, sampleRate int, quiet bool) (*TranscriptionResult, error) {
	// Save raw audio as temporary WAV file
	tmpFile, err := os.CreateTemp("", "whisper-raw-*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Convert raw audio to WAV format
	if err := SaveRawAsWAV(tmpFile.Name(), audioData, sampleRate, 1); err != nil {
		return nil, fmt.Errorf("failed to save WAV file: %w", err)
	}

	// Transcribe the file
	return w.TranscribeFileQuiet(tmpFile.Name(), quiet)
}

// SaveRawAsWAV saves raw int16 audio data as a WAV file
func SaveRawAsWAV(filename string, data []int16, sampleRate int, channels int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// WAV file header
	numSamples := len(data)
	byteRate := sampleRate * channels * 2
	blockAlign := channels * 2
	dataSize := numSamples * channels * 2
	totalSize := 36 + dataSize

	// RIFF header
	file.WriteString("RIFF")
	binary.Write(file, binary.LittleEndian, uint32(totalSize))
	file.WriteString("WAVE")

	// fmt chunk
	file.WriteString("fmt ")
	binary.Write(file, binary.LittleEndian, uint32(16)) // PCM chunk size
	binary.Write(file, binary.LittleEndian, uint16(1))  // PCM format
	binary.Write(file, binary.LittleEndian, uint16(channels))
	binary.Write(file, binary.LittleEndian, uint32(sampleRate))
	binary.Write(file, binary.LittleEndian, uint32(byteRate))
	binary.Write(file, binary.LittleEndian, uint16(blockAlign))
	binary.Write(file, binary.LittleEndian, uint16(16)) // bits per sample

	// data chunk
	file.WriteString("data")
	binary.Write(file, binary.LittleEndian, uint32(dataSize))

	// Write audio data
	for _, sample := range data {
		binary.Write(file, binary.LittleEndian, sample)
	}

	return nil
}
