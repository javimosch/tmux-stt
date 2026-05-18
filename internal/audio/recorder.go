package audio

import (
	"fmt"
	"time"

	"github.com/gordonklaus/portaudio"
)

type SimpleRecorder struct {
	stream *portaudio.Stream
	buffer []int16
	config RecorderConfig
	quiet  bool
}

func NewSimpleRecorder(config RecorderConfig) (*SimpleRecorder, error) {
	return NewSimpleRecorderQuiet(config, false)
}

func NewSimpleRecorderQuiet(config RecorderConfig, quiet bool) (*SimpleRecorder, error) {
	// Initialize PortAudio
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize PortAudio: %w", err)
	}

	// Set default values if not provided
	if config.SampleRate == 0 {
		config.SampleRate = defaultSampleRate
	}
	if config.Channels == 0 {
		config.Channels = defaultChannels
	}
	if config.ChunkSize == 0 {
		config.ChunkSize = defaultChunkSize
	}

	// Get input device
	var device *portaudio.DeviceInfo
	var err error

	if config.InputDevice == "default" {
		device, err = portaudio.DefaultInputDevice()
		if err != nil {
			portaudio.Terminate()
			return nil, fmt.Errorf("failed to get default input device: %w", err)
		}
	} else {
		// Find device by name
		devices, err := portaudio.Devices()
		if err != nil {
			portaudio.Terminate()
			return nil, fmt.Errorf("failed to get devices: %w", err)
		}
		for _, d := range devices {
			if d.Name == config.InputDevice && d.MaxInputChannels > 0 {
				device = d
				break
			}
		}
		if device == nil {
			portaudio.Terminate()
			return nil, fmt.Errorf("input device '%s' not found", config.InputDevice)
		}
	}

	if !quiet {
		fmt.Printf("Using audio device: %s\n", device.Name)
		fmt.Printf("Sample rate: %d Hz, Channels: %d\n", config.SampleRate, config.Channels)
	}

	// Create audio buffer
	buffer := make([]int16, config.ChunkSize)

	// Open stream
	streamParams := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   device,
			Channels: config.Channels,
			Latency:  time.Duration(float64(config.ChunkSize) / float64(config.SampleRate) * float64(time.Second)),
		},
		Output: portaudio.StreamDeviceParameters{
			Device:   nil,
			Channels: 0,
		},
		SampleRate:      float64(config.SampleRate),
		FramesPerBuffer: config.ChunkSize,
	}

	stream, err := portaudio.OpenStream(streamParams, buffer)
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("failed to open audio stream: %w", err)
	}

	return &SimpleRecorder{
		stream: stream,
		buffer: buffer,
		config: config,
		quiet:  quiet,
	}, nil
}

func (r *SimpleRecorder) Start() error {
	if err := r.stream.Start(); err != nil {
		return fmt.Errorf("failed to start audio stream: %w", err)
	}
	return nil
}

func (r *SimpleRecorder) Stop() error {
	if err := r.stream.Stop(); err != nil {
		return fmt.Errorf("failed to stop audio stream: %w", err)
	}
	return nil
}

func (r *SimpleRecorder) Close() error {
	if err := r.stream.Close(); err != nil {
		return fmt.Errorf("failed to close audio stream: %w", err)
	}
	if err := portaudio.Terminate(); err != nil {
		return fmt.Errorf("failed to terminate PortAudio: %w", err)
	}
	return nil
}

func (r *SimpleRecorder) RecordChunk() ([]int16, error) {
	if err := r.stream.Read(); err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Copy the buffer data
	chunk := make([]int16, len(r.buffer))
	copy(chunk, r.buffer)
	
	return chunk, nil
}

func (r *SimpleRecorder) RecordDuration(duration time.Duration) ([]int16, error) {
	chunksNeeded := int(duration.Seconds() * float64(r.config.SampleRate) / float64(r.config.ChunkSize))
	var allData []int16
	
	if !r.quiet {
		fmt.Printf("Recording for %v (%d chunks)...\n", duration, chunksNeeded)
	}
	
	for i := 0; i < chunksNeeded; i++ {
		chunk, err := r.RecordChunk()
		if err != nil {
			return nil, err
		}
		allData = append(allData, chunk...)
		
		// Progress indicator
		if !r.quiet && i%10 == 0 {
			fmt.Printf("Recording progress: %d/%d chunks\n", i, chunksNeeded)
		}
	}
	
	if !r.quiet {
		fmt.Printf("Recording complete: %d samples\n", len(allData))
	}
	return allData, nil
}

func (r *SimpleRecorder) RecordUntilSilence(maxDuration time.Duration, threshold int16, minSilenceChunks int) ([]int16, error) {
	var allData []int16
	silenceChunks := 0
	maxChunks := int(maxDuration.Seconds() * float64(r.config.SampleRate) / float64(r.config.ChunkSize))
	
	fmt.Printf("Recording until silence (max %v, threshold %d)...\n", maxDuration, threshold)
	
	for i := 0; i < maxChunks; i++ {
		chunk, err := r.RecordChunk()
		if err != nil {
			return nil, err
		}
		
		// Check for silence
		maxAmplitude := int16(0)
		for _, sample := range chunk {
			if sample < 0 {
				sample = -sample
			}
			if sample > maxAmplitude {
				maxAmplitude = sample
			}
		}
		
		if maxAmplitude < threshold {
			silenceChunks++
			if silenceChunks >= minSilenceChunks {
				fmt.Printf("Silence detected after %d chunks\n", i)
				break
			}
		} else {
			silenceChunks = 0
			allData = append(allData, chunk...)
		}
		
		if i%10 == 0 {
			fmt.Printf("Recording: %d/%d chunks, current amplitude: %d\n", i, maxChunks, maxAmplitude)
		}
	}
	
	fmt.Printf("Recording complete: %d samples\n", len(allData))
	return allData, nil
}
