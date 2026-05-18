package wakeword

import (
	"fmt"
	"os"
	"path/filepath"
	
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SileroVAD wraps sherpa-onnx Silero VAD for voice activity detection
type SileroVAD struct {
	detector *sherpa.VoiceActivityDetector
	config   SileroVADConfig
}

type SileroVADConfig struct {
	ModelPath           string  // Path to silero_vad.onnx model
	Threshold           float32 // Speech probability threshold (0.0-1.0)
	MinSilenceDuration  float32 // Minimum silence duration in seconds
	MinSpeechDuration   float32 // Minimum speech duration in seconds
	WindowSize          int     // Window size in samples
	SampleRate          int     // Sample rate in Hz
	BufferSizeSeconds   float32 // Buffer size in seconds
}

func DefaultSileroVADConfig() SileroVADConfig {
	homeDir, _ := os.UserHomeDir()
	return SileroVADConfig{
		ModelPath:          filepath.Join(homeDir, ".local", "share", "tmux-stt", "models", "silero_vad.onnx"),
		Threshold:          0.5,
		MinSilenceDuration: 0.5,  // 500ms
		MinSpeechDuration:  0.25, // 250ms
		WindowSize:         512,
		SampleRate:         16000,
		BufferSizeSeconds:  5.0,
	}
}

func NewSileroVAD(config SileroVADConfig) (*SileroVAD, error) {
	// Validate model file exists
	if _, err := os.Stat(config.ModelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("silero VAD model not found at %s", config.ModelPath)
	}

	// Set defaults if not provided
	if config.Threshold == 0 {
		config.Threshold = 0.5
	}
	if config.MinSilenceDuration == 0 {
		config.MinSilenceDuration = 0.5
	}
	if config.MinSpeechDuration == 0 {
		config.MinSpeechDuration = 0.25
	}
	if config.WindowSize == 0 {
		config.WindowSize = 512
	}
	if config.SampleRate == 0 {
		config.SampleRate = 16000
	}
	if config.BufferSizeSeconds == 0 {
		config.BufferSizeSeconds = 5.0
	}

	// Create sherpa-onnx VAD configuration
	vadConfig := sherpa.VadModelConfig{
		SileroVad: sherpa.SileroVadModelConfig{
			Model:              config.ModelPath,
			Threshold:          config.Threshold,
			MinSilenceDuration: config.MinSilenceDuration,
			MinSpeechDuration:  config.MinSpeechDuration,
			WindowSize:         config.WindowSize,
		},
		SampleRate: config.SampleRate,
		NumThreads: 1,
		Provider:   "cpu",
		Debug:      0,
	}

	// Create VAD detector
	detector := sherpa.NewVoiceActivityDetector(&vadConfig, config.BufferSizeSeconds)
	if detector == nil {
		return nil, fmt.Errorf("failed to create silero VAD detector")
	}

	return &SileroVAD{
		detector: detector,
		config:   config,
	}, nil
}

// AcceptWaveform processes audio data through the VAD
func (v *SileroVAD) AcceptWaveform(samples []float32) {
	v.detector.AcceptWaveform(samples)
}

// IsSpeech returns true if speech is currently detected
func (v *SileroVAD) IsSpeech() bool {
	return v.detector.IsSpeech()
}

// IsEmpty returns true if the VAD buffer is empty
func (v *SileroVAD) IsEmpty() bool {
	return v.detector.IsEmpty()
}

// Front returns the next speech segment from the buffer
func (v *SileroVAD) Front() *sherpa.SpeechSegment {
	return v.detector.Front()
}

// Pop removes the front speech segment from the buffer
func (v *SileroVAD) Pop() {
	v.detector.Pop()
}

// Reset clears the VAD state
func (v *SileroVAD) Reset() {
	v.detector.Reset()
}

// Flush processes any remaining buffered audio
func (v *SileroVAD) Flush() {
	v.detector.Flush()
}

// Destroy releases the VAD resources
func (v *SileroVAD) Destroy() {
	if v.detector != nil {
		sherpa.DeleteVoiceActivityDetector(v.detector)
		v.detector = nil
	}
}

// DetectSpeechSegments converts int16 audio to float32 and processes through Silero VAD
func (v *SileroVAD) DetectSpeechSegments(data []int16) []SpeechSegment {
	// Convert int16 to float32
	samples := make([]float32, len(data))
	for i, sample := range data {
		samples[i] = float32(sample) / 32768.0
	}

	// Process through VAD
	v.AcceptWaveform(samples)

	// Extract speech segments
	var segments []SpeechSegment
	for !v.IsEmpty() {
		segment := v.Front()
		if segment != nil {
			// Convert float32 samples back to int16
			int16Data := make([]int16, len(segment.Samples))
			for i, sample := range segment.Samples {
				// Clamp to int16 range
				val := int32(sample * 32768.0)
				if val > 32767 {
					val = 32767
				} else if val < -32768 {
					val = -32768
				}
				int16Data[i] = int16(val)
			}

			segments = append(segments, SpeechSegment{
				Data: int16Data,
			})
		}
		v.Pop()
	}

	return segments
}

// Close is an alias for Destroy for compatibility
func (v *SileroVAD) Close() {
	v.Destroy()
}
