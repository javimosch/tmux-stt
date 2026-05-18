package server

import (
	"fmt"
	"log"
	"time"

	"github.com/javimosch/tmux-stt/internal/audio"
	"github.com/javimosch/tmux-stt/internal/config"
	"github.com/javimosch/tmux-stt/internal/stt"
	"github.com/javimosch/tmux-stt/internal/tmux"
	"github.com/javimosch/tmux-stt/internal/translate"
	"github.com/javimosch/tmux-stt/internal/wakeword"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type Server struct {
	config       *config.Config
	audio        *audio.SimpleRecorder
	sttEngine    *stt.WhisperBinary
	translator   *translate.OpenAITranslator
	wakeDetector *wakeword.WakeWordDetector
	sttAdapter   *wakeword.WhisperBinaryAdapter
	tmuxClient   *tmux.TmuxClient
}

func NewServer(cfg *config.Config, whisperBinaryPath string) (*Server, error) {
	// Initialize audio recorder
	audioConfig := audio.RecorderConfig{
		InputDevice: cfg.Audio.InputDevice,
		SampleRate:  cfg.Audio.SampleRate,
		Channels:    cfg.Audio.Channels,
		ChunkSize:   cfg.Audio.ChunkSize,
	}
	recorder, err := audio.NewSimpleRecorder(audioConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio: %w", err)
	}

	// Initialize STT engine with language support
	sttEngine, err := stt.NewWhisperBinaryWithLanguage(cfg.ModelsDir, cfg.STT.ModelSize, cfg.STT.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize STT: %w", err)
	}

	// Initialize translator
	translator := translate.NewOpenAITranslator(
		cfg.Translation.APIKey,
		cfg.Translation.APIEndpoint,
		cfg.Translation.Model,
	)

	// Create STT adapter for wake word detector
	sttAdapter := wakeword.NewWhisperBinaryAdapter(sttEngine)

	// Initialize wake word detector with configurable VAD parameters
	wakeWordConfig := wakeword.WakeWordConfig{
		WakeWord:    cfg.WakeWord,
		SampleRate:  cfg.Audio.SampleRate,
		ChunkSize:   cfg.Audio.ChunkSize,
		SilenceMs:   cfg.VAD.SilenceMs,
		MinSpeechMs: cfg.VAD.SpeechMs,
		Threshold:   int16(cfg.VAD.Threshold),
	}
	wakeDetector, err := wakeword.NewWakeWordDetector(wakeWordConfig, sttAdapter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize wake word detector: %w", err)
	}

	// Initialize tmux client
	tmuxClient := tmux.NewTmuxClient(cfg.Tmux.SocketPath, cfg.Tmux.AutoEnter)
	tmuxClient.SetTargetPane(cfg.Tmux.TargetPane)
	tmuxClient.SetTargetWindow(cfg.Tmux.TargetWindow)

	return &Server{
		config:       cfg,
		audio:        recorder,
		sttEngine:    sttEngine,
		translator:   translator,
		wakeDetector: wakeDetector,
		sttAdapter:   sttAdapter,
		tmuxClient:   tmuxClient,
	}, nil
}

func (s *Server) Start() error {
	log.Println("🎤 Starting tmux-voice-to-text server...")
	log.Printf("Wake word: '%s'", s.config.WakeWord)
	log.Printf("Translation: %v", s.config.Translation.Enabled)
	log.Printf("Target language: %s", s.config.Translation.TargetLang)

	// Check if running inside tmux
	if !s.tmuxClient.IsInsideTmux() {
		log.Println("⚠️  Not running inside tmux")
		log.Println("Text will be logged but not sent to tmux")
		log.Println("Run this command inside a tmux session for full functionality")
	}

	// Start audio recording
	if err := s.audio.Start(); err != nil {
		return fmt.Errorf("failed to start audio: %w", err)
	}
	defer s.audio.Stop()

	// Main listening loop
	for {
		log.Println("👂 Listening for wake word...")

		// Record until silence is detected (max 20 seconds)
		silenceDuration := time.Duration(s.config.VAD.SilenceMs) * time.Millisecond
		minSilenceChunks := max(1, int(silenceDuration.Milliseconds())*s.config.Audio.SampleRate/(s.config.Audio.ChunkSize*1000)/2) // At least 1 chunk, roughly half the duration
		
		audioData, err := s.audio.RecordUntilSilence(20*time.Second, int16(s.config.VAD.Threshold), minSilenceChunks)
		if err != nil {
			log.Printf("❌ Error recording audio: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Skip if no speech was detected (empty or very short recording)
		minSpeechSamples := s.config.Audio.SampleRate * s.config.VAD.SpeechMs / 1000
		if len(audioData) < minSpeechSamples {
			log.Println("⏱️  No speech detected, listening again...")
			continue
		}

		// Detect wake word in recorded audio
		detected, transcription, err := s.wakeDetector.DetectWakeWordInAudio(audioData)
		if err != nil {
			log.Printf("❌ Error detecting wake word: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if !detected {
			log.Println("⏱️  Wake word not detected, listening again...")
			continue
		}

		log.Printf("✅ Wake word detected! Transcription: '%s'", transcription)

		// Strip wake word from transcription if configured
		if s.config.StripWakeWord {
			transcription = wakeword.StripWakeWord(transcription, s.config.WakeWord)
			if transcription != "" {
				log.Printf("🔪 Stripped wake word: '%s'", transcription)
			}
		}

		// Apply translation if enabled
		if s.config.Translation.Enabled && s.config.Translation.APIKey != "" {
			log.Printf("🌐 Translating to %s...", s.config.Translation.TargetLang)
			
			translated, err := s.translator.Translate(transcription, s.config.Translation.TargetLang)
			if err != nil {
				log.Printf("❌ Translation error: %v", err)
				log.Printf("📝 Original text: '%s'", transcription)
			} else {
				log.Printf("📝 Translated: '%s'", translated)
				transcription = translated
			}
		}

		// Send to tmux
		if s.tmuxClient.IsInsideTmux() {
			if s.config.Tmux.AutoEnter {
				log.Printf("📤 Sending to tmux (with Enter): '%s'", transcription)
				if err := s.tmuxClient.SendKeys(transcription); err != nil {
					log.Printf("❌ Failed to send to tmux: %v", err)
				}
			} else {
				log.Printf("📤 Sending to tmux (no Enter): '%s'", transcription)
				if err := s.tmuxClient.SendKeysNoEnter(transcription); err != nil {
					log.Printf("❌ Failed to send to tmux: %v", err)
				}
			}
		} else {
			log.Printf("📝 Not in tmux, would send: '%s'", transcription)
		}

		// Brief pause before listening again
		time.Sleep(1 * time.Second)
	}
}

func (s *Server) Stop() error {
	log.Println("🛑 Stopping server...")
	
	if err := s.audio.Stop(); err != nil {
		return fmt.Errorf("failed to stop audio: %w", err)
	}
	
	if err := s.audio.Close(); err != nil {
		return fmt.Errorf("failed to close audio: %w", err)
	}

	return nil
}
