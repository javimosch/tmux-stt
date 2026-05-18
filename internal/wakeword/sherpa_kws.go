package wakeword

import (
	"fmt"
	"os"
	"path/filepath"
	
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SherpaKWS wraps sherpa-onnx keyword spotting for wake word detection
type SherpaKWS struct {
	spotter *sherpa.KeywordSpotter
	config  SherpaKWSConfig
}

type SherpaKWSConfig struct {
	ModelDir     string // Directory containing KWS model files
	EncoderPath  string // Path to encoder ONNX model
	DecoderPath  string // Path to decoder ONNX model
	JoinerPath   string // Path to joiner ONNX model
	TokensPath  string // Path to tokens.txt file
	KeywordsFile string // Path to keywords.txt file
	SampleRate   int    // Sample rate in Hz
	NumThreads   int    // Number of threads for inference
}

func DefaultSherpaKWSConfig() SherpaKWSConfig {
	homeDir, _ := os.UserHomeDir()
	modelDir := filepath.Join(homeDir, ".local", "share", "tmux-stt", "sherpa-kws")
	return SherpaKWSConfig{
		ModelDir:     modelDir,
		EncoderPath:  filepath.Join(modelDir, "encoder-epoch-13-avg-2-chunk-16-left-64.onnx"),
		DecoderPath:  filepath.Join(modelDir, "decoder-epoch-13-avg-2-chunk-16-left-64.onnx"),
		JoinerPath:   filepath.Join(modelDir, "joiner-epoch-13-avg-2-chunk-16-left-64.onnx"),
		TokensPath:   filepath.Join(modelDir, "tokens.txt"),
		KeywordsFile: filepath.Join(modelDir, "keywords.txt"),
		SampleRate:   16000,
		NumThreads:   1,
	}
}

func NewSherpaKWS(config SherpaKWSConfig, keywords []string) (*SherpaKWS, error) {
	// Validate model files exist
	for _, path := range []string{config.EncoderPath, config.DecoderPath, config.JoinerPath, config.TokensPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("KWS model file not found: %s", path)
		}
	}

	// Set defaults if not provided
	if config.SampleRate == 0 {
		config.SampleRate = 16000
	}
	if config.NumThreads == 0 {
		config.NumThreads = 1
	}

	// Create keywords file if it doesn't exist
	if _, err := os.Stat(config.KeywordsFile); os.IsNotExist(err) {
		if len(keywords) == 0 {
			return nil, fmt.Errorf("no keywords provided and keywords file does not exist")
		}
		if err := createKeywordsFile(config.KeywordsFile, keywords); err != nil {
			return nil, fmt.Errorf("failed to create keywords file: %w", err)
		}
	}

	// Create sherpa-onnx keyword spotter configuration
	kwsConfig := sherpa.KeywordSpotterConfig{
		ModelConfig: sherpa.OnlineModelConfig{
			Transducer: sherpa.OnlineTransducerModelConfig{
				Encoder: config.EncoderPath,
				Decoder: config.DecoderPath,
				Joiner:  config.JoinerPath,
			},
			Tokens:     config.TokensPath,
			NumThreads: config.NumThreads,
			Provider:   "cpu",
			Debug:      0,
		},
		KeywordsFile: config.KeywordsFile,
	}

	// Create keyword spotter
	spotter := sherpa.NewKeywordSpotter(&kwsConfig)
	if spotter == nil {
		return nil, fmt.Errorf("failed to create sherpa keyword spotter")
	}

	return &SherpaKWS{
		spotter: spotter,
		config:  config,
	}, nil
}

// createKeywordsFile creates a keywords file for sherpa-onnx KWS
func createKeywordsFile(path string, keywords []string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create keywords file
	content := ""
	for i, keyword := range keywords {
		if i > 0 {
			content += "\n"
		}
		content += fmt.Sprintf("%s @%s", keyword, keyword)
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// ProcessAudio processes audio data and detects keywords
func (k *SherpaKWS) ProcessAudio(samples []float32) (string, bool) {
	// Create a new stream for this processing operation
	stream := sherpa.NewKeywordStream(k.spotter)
	if stream == nil {
		return "", false
	}

	// Accept waveform into stream
	stream.AcceptWaveform(k.config.SampleRate, samples)

	// Decode if ready
	for k.spotter.IsReady(stream) {
		k.spotter.Decode(stream)
		result := k.spotter.GetResult(stream)

		if result.Keyword != "" {
			// Keyword detected
			k.spotter.Reset(stream)
			return result.Keyword, true
		}
	}

	return "", false
}

// SetKeywords dynamically updates the keywords
func (k *SherpaKWS) SetKeywords(keywords []string) error {
	// Create new keywords file
	if err := createKeywordsFile(k.config.KeywordsFile, keywords); err != nil {
		return err
	}

	// Recreate spotter with new keywords
	sherpa.DeleteKeywordSpotter(k.spotter)
	
	kwsConfig := sherpa.KeywordSpotterConfig{
		ModelConfig: sherpa.OnlineModelConfig{
			Transducer: sherpa.OnlineTransducerModelConfig{
				Encoder: k.config.EncoderPath,
				Decoder: k.config.DecoderPath,
				Joiner:  k.config.JoinerPath,
			},
			Tokens:     k.config.TokensPath,
			NumThreads: k.config.NumThreads,
			Provider:   "cpu",
			Debug:      0,
		},
		KeywordsFile: k.config.KeywordsFile,
	}
	
	k.spotter = sherpa.NewKeywordSpotter(&kwsConfig)
	if k.spotter == nil {
		return fmt.Errorf("failed to recreate keyword spotter")
	}

	return nil
}

// Destroy releases the keyword spotter resources
func (k *SherpaKWS) Destroy() {
	if k.spotter != nil {
		sherpa.DeleteKeywordSpotter(k.spotter)
		k.spotter = nil
	}
}

// Close is an alias for Destroy for compatibility
func (k *SherpaKWS) Close() {
	k.Destroy()
}
