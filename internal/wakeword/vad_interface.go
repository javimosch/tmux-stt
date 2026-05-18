package wakeword

import (
	"fmt"
	
	"github.com/javimosch/tmux-stt/internal/config"
)

// VADInterface defines the common interface for voice activity detection
type VADInterface interface {
	DetectSpeechSegments(data []int16) []SpeechSegment
	IsSpeech(data []int16) bool
	Close()
}

// VADFactory creates a VAD implementation based on configuration
func VADFactory(cfg *config.Config) (VADInterface, error) {
	switch cfg.VAD.Method {
	case "silero", "sherpa":
		// Create Silero VAD
		sileroConfig := SileroVADConfig{
			ModelPath:          cfg.VAD.Silero.ModelPath,
			Threshold:          cfg.VAD.Silero.Threshold,
			MinSilenceDuration: cfg.VAD.Silero.MinSilenceDuration,
			MinSpeechDuration:  cfg.VAD.Silero.MinSpeechDuration,
			WindowSize:         cfg.VAD.Silero.WindowSize,
			SampleRate:         cfg.Audio.SampleRate,
			BufferSizeSeconds:  5.0,
		}
		
		sileroVAD, err := NewSileroVAD(sileroConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create silero VAD: %w", err)
		}
		
		// Wrap Silero VAD to implement the interface
		return &SileroVADWrapper{sileroVAD: sileroVAD}, nil
		
	case "energy", "":
		// Create energy-based VAD (default)
		vadConfig := VADConfig{
			Threshold:  int16(cfg.VAD.Threshold),
			SampleRate: cfg.Audio.SampleRate,
			Channels:   cfg.Audio.Channels,
			SilenceMs:  cfg.VAD.SilenceMs,
			SpeechMs:   cfg.VAD.SpeechMs,
		}
		
		return &EnergyVADWrapper{vad: NewVAD(vadConfig)}, nil
		
	default:
		return nil, fmt.Errorf("unknown VAD method: %s", cfg.VAD.Method)
	}
}

// EnergyVADWrapper wraps the energy-based VAD to implement VADInterface
type EnergyVADWrapper struct {
	vad *VAD
}

func (w *EnergyVADWrapper) DetectSpeechSegments(data []int16) []SpeechSegment {
	chunkSize := 1024 // Default chunk size
	return w.vad.DetectSpeechSegments(data, chunkSize)
}

func (w *EnergyVADWrapper) IsSpeech(data []int16) bool {
	return w.vad.IsSpeech(data)
}

func (w *EnergyVADWrapper) Close() {
	// Energy VAD doesn't need cleanup
}

// SileroVADWrapper wraps the Silero VAD to implement VADInterface
type SileroVADWrapper struct {
	sileroVAD *SileroVAD
}

func (w *SileroVADWrapper) DetectSpeechSegments(data []int16) []SpeechSegment {
	return w.sileroVAD.DetectSpeechSegments(data)
}

func (w *SileroVADWrapper) IsSpeech(data []int16) bool {
	// For Silero VAD, we need to convert and check speech
	samples := make([]float32, len(data))
	for i, sample := range data {
		samples[i] = float32(sample) / 32768.0
	}
	
	w.sileroVAD.AcceptWaveform(samples)
	return w.sileroVAD.IsSpeech()
}

func (w *SileroVADWrapper) Close() {
	w.sileroVAD.Destroy()
}
