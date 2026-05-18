package wakeword

import (
	"fmt"
	"strings"
	"time"

	"github.com/javimosch/tmux-stt/internal/stt"
)

type WakeWordDetector struct {
	wakeWord      string
	vad           *VAD
	sttEngine     STTEngine
	sampleRate    int
	chunkSize     int
	silenceMs     int
	minSpeechMs   int
	detectionCallback func(detected bool, transcription string)
}

type STTEngine interface {
	TranscribeRaw(audioData []int16, sampleRate int) (*TranscriptionResult, error)
}

type TranscriptionResult struct {
	Text     string
	Language string
	Duration float64
}

// WhisperBinaryAdapter adapts WhisperBinary to STTEngine interface
type WhisperBinaryAdapter struct {
	binary *stt.WhisperBinary
	quiet  bool
}

func NewWhisperBinaryAdapter(binary *stt.WhisperBinary) *WhisperBinaryAdapter {
	return &WhisperBinaryAdapter{binary: binary, quiet: false}
}

func NewWhisperBinaryAdapterQuiet(binary *stt.WhisperBinary) *WhisperBinaryAdapter {
	return &WhisperBinaryAdapter{binary: binary, quiet: true}
}

func (w *WhisperBinaryAdapter) TranscribeRaw(audioData []int16, sampleRate int) (*TranscriptionResult, error) {
	var result *stt.TranscriptionResult
	var err error
	
	if w.quiet {
		result, err = w.binary.TranscribeRawQuiet(audioData, sampleRate, true)
	} else {
		result, err = w.binary.TranscribeRaw(audioData, sampleRate)
	}
	
	if err != nil {
		return nil, err
	}

	return &TranscriptionResult{
		Text:     result.Text,
		Language: result.Language,
		Duration: result.Duration,
	}, nil
}

type WakeWordConfig struct {
	WakeWord    string
	SampleRate  int
	ChunkSize   int
	SilenceMs   int
	MinSpeechMs int
	Threshold   int16
}

func DefaultWakeWordConfig() WakeWordConfig {
	return WakeWordConfig{
		WakeWord:    "Hey mate",
		SampleRate:  16000,
		ChunkSize:   1024,
		SilenceMs:   1000,
		MinSpeechMs: 500,
		Threshold:   500,
	}
}

func NewWakeWordDetector(config WakeWordConfig, sttEngine STTEngine) (*WakeWordDetector, error) {
	vadConfig := VADConfig{
		SampleRate: config.SampleRate,
		Channels:   1,
		SilenceMs:  config.SilenceMs,
		SpeechMs:   config.MinSpeechMs,
		Threshold:  config.Threshold,
	}

	return &WakeWordDetector{
		wakeWord:    strings.ToLower(config.WakeWord),
		vad:         NewVAD(vadConfig),
		sttEngine:   sttEngine,
		sampleRate:  config.SampleRate,
		chunkSize:    config.ChunkSize,
		silenceMs:    config.SilenceMs,
		minSpeechMs:  config.MinSpeechMs,
	}, nil
}

func (w *WakeWordDetector) SetDetectionCallback(callback func(detected bool, transcription string)) {
	w.detectionCallback = callback
}

// MonitorAudio continuously monitors audio for wake word
func (w *WakeWordDetector) MonitorAudio(audioData []int16) bool {
	// Detect speech segments
	segments := w.vad.DetectSpeechSegments(audioData, w.chunkSize)
	
	if len(segments) == 0 {
		return false
	}

	// Transcribe each speech segment and check for wake word
	for _, segment := range segments {
		transcription, err := w.sttEngine.TranscribeRaw(segment.Data, w.sampleRate)
		if err != nil {
			fmt.Printf("Transcription error: %v\n", err)
			continue
		}

		// Check if wake word is in transcription
		if w.containsWakeWord(transcription.Text) {
			if w.detectionCallback != nil {
				w.detectionCallback(true, transcription.Text)
			}
			return true
		}
	}

	return false
}

// MonitorContinuous continuously monitors audio stream for wake word
func (w *WakeWordDetector) MonitorContinuous(getAudioChunk func() ([]int16, error), duration time.Duration) bool {
	endTime := time.Now().Add(duration)
	buffer := make([]int16, 0)
	maxBufferSize := w.sampleRate * 10 // Keep last 10 seconds

	for time.Now().Before(endTime) {
		chunk, err := getAudioChunk()
		if err != nil {
			fmt.Printf("Error getting audio chunk: %v\n", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Add to buffer
		buffer = append(buffer, chunk...)
		
		// Keep buffer size manageable
		if len(buffer) > maxBufferSize {
			buffer = buffer[len(buffer)-maxBufferSize:]
		}

		// Try to detect wake word in current buffer
		if w.MonitorAudio(buffer) {
			return true
		}
	}

	return false
}

// containsWakeWord checks if the wake word appears in the transcription
func (w *WakeWordDetector) containsWakeWord(transcription string) bool {
	lowerTranscription := strings.ToLower(transcription)
	lowerWakeWord := strings.ToLower(w.wakeWord)
	
	// Check for exact match
	if strings.Contains(lowerTranscription, lowerWakeWord) {
		return true
	}

	// Check for word boundaries
	words := strings.Fields(lowerTranscription)
	for _, word := range words {
		if strings.Contains(word, lowerWakeWord) || strings.Contains(lowerWakeWord, word) {
			return true
		}
	}

	return false
}

// DetectWakeWordInAudio detects wake word in a single audio clip
func (w *WakeWordDetector) DetectWakeWordInAudio(audioData []int16) (bool, string, error) {
	transcription, err := w.sttEngine.TranscribeRaw(audioData, w.sampleRate)
	if err != nil {
		return false, "", err
	}

	detected := w.containsWakeWord(transcription.Text)
	return detected, transcription.Text, nil
}

// SetWakeWord changes the wake word
func (w *WakeWordDetector) SetWakeWord(wakeWord string) {
	w.wakeWord = strings.ToLower(wakeWord)
}

// GetWakeWord returns the current wake word
func (w *WakeWordDetector) GetWakeWord() string {
	return w.wakeWord
}

// StripWakeWord removes the wake word from the beginning of text
func StripWakeWord(text, wakeWord string) string {
	if wakeWord == "" {
		return text
	}

	lowerText := strings.ToLower(strings.TrimSpace(text))
	lowerWakeWord := strings.ToLower(strings.TrimSpace(wakeWord))

	// Check if text starts with wake word
	if strings.HasPrefix(lowerText, lowerWakeWord) {
		// Remove the wake word and any following whitespace/punctuation
		remaining := strings.TrimSpace(text[len(wakeWord):])
		// Remove common punctuation after wake word
		remaining = strings.TrimLeft(remaining, ",.!?;:")
		return strings.TrimSpace(remaining)
	}

	// Check if wake word appears as first word
	words := strings.Fields(text)
	if len(words) > 0 {
		firstWord := strings.ToLower(strings.Trim(words[0], ",.!?;:"))
		if firstWord == lowerWakeWord {
			// Remove first word and join the rest
			remaining := strings.Join(words[1:], " ")
			return strings.TrimSpace(remaining)
		}
	}

	return text
}
