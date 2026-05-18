package wakeword

import (
	"math"
)

// VAD (Voice Activity Detection) using energy threshold
type VAD struct {
	threshold   int16  // Energy threshold for speech detection
	sampleRate  int    // Sample rate in Hz
	channels    int    // Number of audio channels
	silenceMs   int    // Silence duration in milliseconds to consider speech ended
	speechMs    int    // Minimum speech duration in milliseconds
}

type VADConfig struct {
	Threshold   int16 // Energy threshold (default: 500)
	SampleRate  int   // Sample rate (default: 16000)
	Channels    int   // Channels (default: 1)
	SilenceMs   int   // Silence duration (default: 1000ms)
	SpeechMs    int   // Minimum speech duration (default: 500ms)
}

func DefaultVADConfig() VADConfig {
	return VADConfig{
		Threshold:  500,
		SampleRate: 16000,
		Channels:   1,
		SilenceMs:  1000,
		SpeechMs:   500,
	}
}

func NewVAD(config VADConfig) *VAD {
	if config.Threshold == 0 {
		config.Threshold = 500
	}
	if config.SampleRate == 0 {
		config.SampleRate = 16000
	}
	if config.Channels == 0 {
		config.Channels = 1
	}
	if config.SilenceMs == 0 {
		config.SilenceMs = 1000
	}
	if config.SpeechMs == 0 {
		config.SpeechMs = 500
	}

	return &VAD{
		threshold:  config.Threshold,
		sampleRate: config.SampleRate,
		channels:   config.Channels,
		silenceMs:  config.SilenceMs,
		speechMs:   config.SpeechMs,
	}
}

// Calculate energy (RMS) of audio chunk
func (v *VAD) calculateEnergy(data []int16) float64 {
	if len(data) == 0 {
		return 0
	}

	sum := int64(0)
	for _, sample := range data {
		sum += int64(sample * sample)
	}

	rms := math.Sqrt(float64(sum) / float64(len(data)))
	return rms
}

// DetectSpeechSegments detects speech segments in audio data
func (v *VAD) DetectSpeechSegments(data []int16, chunkSize int) []SpeechSegment {
	chunksNeeded := len(data) / chunkSize
	var segments []SpeechSegment
	inSpeech := false
	silenceChunks := 0
	speechChunks := 0
	currentSegmentStart := 0

	minSilenceChunks := v.silenceMs * v.sampleRate / (chunkSize * 1000)
	minSpeechChunks := v.speechMs * v.sampleRate / (chunkSize * 1000)

	for i := 0; i < chunksNeeded; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[start:end]
		energy := v.calculateEnergy(chunk)

		amplitude := int16(energy)
		isSpeech := amplitude > v.threshold

		if isSpeech {
			if !inSpeech {
				inSpeech = true
				currentSegmentStart = start
			}
			speechChunks++
			silenceChunks = 0
		} else {
			if inSpeech {
				silenceChunks++
				if silenceChunks >= int(minSilenceChunks) && speechChunks >= int(minSpeechChunks) {
					// End of speech segment
					segments = append(segments, SpeechSegment{
						Start: currentSegmentStart,
						End:   start,
						Data:  data[currentSegmentStart:end],
					})
					inSpeech = false
					speechChunks = 0
				}
			}
		}
	}

	// Handle case where audio ends while still in speech
	if inSpeech && speechChunks >= int(minSpeechChunks) {
		segments = append(segments, SpeechSegment{
			Start: currentSegmentStart,
			End:   len(data),
			Data:  data[currentSegmentStart:],
		})
	}

	return segments
}

// IsSpeech checks if a chunk contains speech
func (v *VAD) IsSpeech(data []int16) bool {
	energy := v.calculateEnergy(data)
	return int16(energy) > v.threshold
}

// GetDynamicThreshold calculates a dynamic threshold based on background noise
func (v *VAD) GetDynamicThreshold(data []int16, sampleSize int) int16 {
	// Calculate energy of first N samples as baseline
	samplesToCheck := min(sampleSize, len(data))
	if samplesToCheck == 0 {
		return v.threshold
	}

	baselineData := data[:samplesToCheck]
	baselineEnergy := v.calculateEnergy(baselineData)
	
	// Set threshold to 3x baseline energy
	dynamicThreshold := int16(baselineEnergy * 3)
	
	// Ensure minimum threshold
	if dynamicThreshold < v.threshold {
		dynamicThreshold = v.threshold
	}
	
	return dynamicThreshold
}

type SpeechSegment struct {
	Start int
	End   int
	Data  []int16
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
