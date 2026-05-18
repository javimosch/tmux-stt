package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ConfigDir      = ".tmux-stt"
	ConfigFile     = "config.json"
	DefaultWakeWord = "Hey mate"
	DefaultModel    = "tiny"
)

type Config struct {
	WakeWord      string            `json:"wake_word"`
	WakeWordMethod string           `json:"wake_word_method"` // "stt" (current) or "sherpa-kws"
	Translation   TranslationConfig `json:"translation"`
	Tmux          TmuxConfig        `json:"tmux"`
	Audio         AudioConfig       `json:"audio"`
	STT           STTConfig         `json:"stt"`
	VAD           VADConfig         `json:"vad"`
	KWS           KWSConfig         `json:"kws"`           // Keyword spotting configuration
	ModelsDir     string            `json:"models_dir"`
	StripWakeWord bool              `json:"strip_wake_word"` // Remove wake word from final output
}

type TranslationConfig struct {
	Enabled     bool   `json:"enabled"`
	SourceLang  string `json:"source_lang"`
	TargetLang  string `json:"target_lang"`
	APIEndpoint string `json:"api_endpoint"`
	APIKey      string `json:"api_key"`
	Model       string `json:"model"`
}

type TmuxConfig struct {
	SocketPath   string `json:"socket_path"`
	AutoEnter    bool   `json:"auto_enter"`
	TargetPane   string `json:"target_pane"`   // Format: "session:window.pane" or "window.pane" or ".pane"
	TargetWindow string `json:"target_window"` // Format: "session:window" or "window"
}

type AudioConfig struct {
	InputDevice   string `json:"input_device"`
	SampleRate    int    `json:"sample_rate"`
	Channels      int    `json:"channels"`
	ChunkSize     int    `json:"chunk_size"`
	SilenceThreshold int16 `json:"silence_threshold"` // Threshold for silence detection
	SilenceDurationMs int `json:"silence_duration_ms"` // How long silence before stopping (ms)
	MinSpeechDurationMs int `json:"min_speech_duration_ms"` // Minimum speech duration to consider valid (ms)
}

type STTConfig struct {
	Language string `json:"language"` // Language for STT (e.g., "es", "en", "auto")
	ModelSize string `json:"model_size"`
}

type VADConfig struct {
	Method      string            `json:"method"`       // "energy" (current) or "silero" (sherpa-onnx)
	SilenceMs   int               `json:"silence_ms"`   // Silence duration in milliseconds
	SpeechMs    int               `json:"speech_ms"`    // Minimum speech duration in milliseconds
	Threshold   int               `json:"threshold"`    // Energy threshold for speech detection (energy method)
	Silero      SileroVADConfig   `json:"silero"`       // Silero VAD configuration
}

type SileroVADConfig struct {
	ModelPath           string  `json:"model_path"`
	Threshold           float32 `json:"threshold"`             // Speech probability threshold (0.0-1.0)
	MinSilenceDuration  float32 `json:"min_silence_duration"`  // Seconds
	MinSpeechDuration   float32 `json:"min_speech_duration"`   // Seconds
	WindowSize          int     `json:"window_size"`           // Samples
}

type KWSConfig struct {
	ModelDir     string `json:"model_dir"`     // Directory containing KWS models
	EncoderPath  string `json:"encoder_path"`  // Path to encoder ONNX model
	DecoderPath  string `json:"decoder_path"`  // Path to decoder ONNX model
	JoinerPath   string `json:"joiner_path"`   // Path to joiner ONNX model
	TokensPath   string `json:"tokens_path"`   // Path to tokens.txt file
	KeywordsFile string `json:"keywords_file"` // Path to keywords.txt file
}

func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	kwsDir := filepath.Join(homeDir, ".local", "share", "tmux-stt", "sherpa-kws")
	return &Config{
		WakeWord:      DefaultWakeWord,
		WakeWordMethod: "stt", // Default to STT-based wake word detection
		StripWakeWord: true, // Default to stripping wake word from output
		Translation: TranslationConfig{
			Enabled:     false,
			SourceLang:  "auto",
			TargetLang:  "en",
			APIEndpoint: "http://localhost:11434/v1", // Default to local Ollama
			Model:       "gpt-4o-mini",                // Default model
		},
		Tmux: TmuxConfig{
			SocketPath:   "",
			AutoEnter:    true,
			TargetPane:   "", // Empty means use active pane
			TargetWindow: "", // Empty means use active window
		},
		Audio: AudioConfig{
			InputDevice:   "default",
			SampleRate:    16000,
			Channels:      1,
			ChunkSize:     1024,
			SilenceThreshold: 500,  // Default threshold for silence detection
			SilenceDurationMs: 600, // Stop after 600ms of silence (was 1000ms)
			MinSpeechDurationMs: 300, // Minimum 300ms of speech (was 500ms)
		},
		STT: STTConfig{
			Language:  "es", // Default to Spanish
			ModelSize: DefaultModel,
		},
		ModelsDir: filepath.Join(homeDir, ".local", "share", "tmux-stt", "models"),
		VAD: VADConfig{
			Method:    "energy", // Default to energy-based VAD
			SilenceMs: 800,     // Wait longer before stopping (was 600ms)
			SpeechMs:  500,     // Require more speech (was 300ms)
			Threshold: 300,     // More sensitive (was 500)
			Silero: SileroVADConfig{
				ModelPath:          filepath.Join(homeDir, ".local", "share", "tmux-stt", "models", "silero_vad.onnx"),
				Threshold:          0.5,  // Speech probability threshold
				MinSilenceDuration: 0.5,  // 500ms silence
				MinSpeechDuration:  0.25, // 250ms speech
				WindowSize:         512,  // Window size in samples
			},
		},
		KWS: KWSConfig{
			ModelDir:     kwsDir,
			EncoderPath:  filepath.Join(kwsDir, "encoder-epoch-13-avg-2-chunk-16-left-64.onnx"),
			DecoderPath:  filepath.Join(kwsDir, "decoder-epoch-13-avg-2-chunk-16-left-64.onnx"),
			JoinerPath:   filepath.Join(kwsDir, "joiner-epoch-13-avg-2-chunk-16-left-64.onnx"),
			TokensPath:   filepath.Join(kwsDir, "tokens.txt"),
			KeywordsFile: filepath.Join(kwsDir, "keywords.txt"),
		},
	}
}

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// If config doesn't exist, return default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

func Save(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ConfigDir, ConfigFile), nil
}

func (c *Config) Set(key, value string) error {
	switch key {
	case "wake-word":
		c.WakeWord = value
	case "wake-word-method":
		c.WakeWordMethod = value
	case "strip-wake-word":
		c.StripWakeWord = value == "true"
	case "stt.language":
		c.STT.Language = value
	case "stt.model":
		c.STT.ModelSize = value
	case "model": // Legacy support
		c.STT.ModelSize = value
	case "translation.enabled":
		c.Translation.Enabled = value == "true"
	case "translation.source":
		c.Translation.SourceLang = value
	case "translation.target":
		c.Translation.TargetLang = value
	case "translation.endpoint":
		c.Translation.APIEndpoint = value
	case "translation.api-key":
		c.Translation.APIKey = value
	case "translation.model":
		c.Translation.Model = value
	case "tmux.socket":
		c.Tmux.SocketPath = value
	case "tmux.auto-enter":
		c.Tmux.AutoEnter = value == "true"
	case "tmux.target-pane":
		c.Tmux.TargetPane = value
	case "tmux.target-window":
		c.Tmux.TargetWindow = value
	case "audio.device":
		c.Audio.InputDevice = value
	case "audio.sample-rate":
		var sr int
		fmt.Sscanf(value, "%d", &sr)
		c.Audio.SampleRate = sr
	case "audio.chunk-size":
		var cs int
		fmt.Sscanf(value, "%d", &cs)
		c.Audio.ChunkSize = cs
	case "audio.silence-threshold":
		var st int
		fmt.Sscanf(value, "%d", &st)
		c.Audio.SilenceThreshold = int16(st)
	case "audio.silence-duration":
		var sd int
		fmt.Sscanf(value, "%d", &sd)
		c.Audio.SilenceDurationMs = sd
	case "audio.min-speech-duration":
		var msd int
		fmt.Sscanf(value, "%d", &msd)
		c.Audio.MinSpeechDurationMs = msd
	case "vad.silence-ms":
		var sm int
		fmt.Sscanf(value, "%d", &sm)
		c.VAD.SilenceMs = sm
	case "vad.speech-ms":
		var sm int
		fmt.Sscanf(value, "%d", &sm)
		c.VAD.SpeechMs = sm
	case "vad.threshold":
		var th int
		fmt.Sscanf(value, "%d", &th)
		c.VAD.Threshold = th
	case "vad.method":
		c.VAD.Method = value
	case "vad.silero.model":
		c.VAD.Silero.ModelPath = value
	case "vad.silero.threshold":
		var th float32
		fmt.Sscanf(value, "%f", &th)
		c.VAD.Silero.Threshold = th
	case "vad.silero.min-silence-duration":
		var msd float32
		fmt.Sscanf(value, "%f", &msd)
		c.VAD.Silero.MinSilenceDuration = msd
	case "vad.silero.min-speech-duration":
		var msd float32
		fmt.Sscanf(value, "%f", &msd)
		c.VAD.Silero.MinSpeechDuration = msd
	case "vad.silero.window-size":
		var ws int
		fmt.Sscanf(value, "%d", &ws)
		c.VAD.Silero.WindowSize = ws
	case "kws.model-dir":
		c.KWS.ModelDir = value
	case "kws.encoder-path":
		c.KWS.EncoderPath = value
	case "kws.decoder-path":
		c.KWS.DecoderPath = value
	case "kws.joiner-path":
		c.KWS.JoinerPath = value
	case "kws.tokens-path":
		c.KWS.TokensPath = value
	case "kws.keywords-file":
		c.KWS.KeywordsFile = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

func (c *Config) Get(key string) (string, error) {
	switch key {
	case "wake-word":
		return c.WakeWord, nil
	case "wake-word-method":
		return c.WakeWordMethod, nil
	case "strip-wake-word":
		return fmt.Sprintf("%v", c.StripWakeWord), nil
	case "stt.language":
		return c.STT.Language, nil
	case "stt.model":
		return c.STT.ModelSize, nil
	case "model": // Legacy support
		return c.STT.ModelSize, nil
	case "translation.enabled":
		return fmt.Sprintf("%v", c.Translation.Enabled), nil
	case "translation.source":
		return c.Translation.SourceLang, nil
	case "translation.target":
		return c.Translation.TargetLang, nil
	case "translation.endpoint":
		return c.Translation.APIEndpoint, nil
	case "translation.model":
		return c.Translation.Model, nil
	case "tmux.socket":
		return c.Tmux.SocketPath, nil
	case "tmux.auto-enter":
		return fmt.Sprintf("%v", c.Tmux.AutoEnter), nil
	case "tmux.target-pane":
		return c.Tmux.TargetPane, nil
	case "tmux.target-window":
		return c.Tmux.TargetWindow, nil
	case "audio.device":
		return c.Audio.InputDevice, nil
	case "audio.min-speech-duration":
		return fmt.Sprintf("%d", c.Audio.MinSpeechDurationMs), nil
	case "vad.silence-ms":
		return fmt.Sprintf("%d", c.VAD.SilenceMs), nil
	case "vad.speech-ms":
		return fmt.Sprintf("%d", c.VAD.SpeechMs), nil
	case "vad.threshold":
		return fmt.Sprintf("%d", c.VAD.Threshold), nil
	case "vad.method":
		return c.VAD.Method, nil
	case "vad.silero.model":
		return c.VAD.Silero.ModelPath, nil
	case "vad.silero.threshold":
		return fmt.Sprintf("%f", c.VAD.Silero.Threshold), nil
	case "vad.silero.min-silence-duration":
		return fmt.Sprintf("%f", c.VAD.Silero.MinSilenceDuration), nil
	case "vad.silero.min-speech-duration":
		return fmt.Sprintf("%f", c.VAD.Silero.MinSpeechDuration), nil
	case "vad.silero.window-size":
		return fmt.Sprintf("%d", c.VAD.Silero.WindowSize), nil
	case "kws.model-dir":
		return c.KWS.ModelDir, nil
	case "kws.encoder-path":
		return c.KWS.EncoderPath, nil
	case "kws.decoder-path":
		return c.KWS.DecoderPath, nil
	case "kws.joiner-path":
		return c.KWS.JoinerPath, nil
	case "kws.tokens-path":
		return c.KWS.TokensPath, nil
	case "kws.keywords-file":
		return c.KWS.KeywordsFile, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}
