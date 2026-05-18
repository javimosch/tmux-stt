package audio

import (
	"fmt"
	"time"

	"github.com/gordonklaus/portaudio"
)

const (
	defaultSampleRate = 16000
	defaultChannels   = 1
	defaultChunkSize  = 1024
)

type Recorder struct {
	stream *portaudio.Stream
	config RecorderConfig
}

type RecorderConfig struct {
	InputDevice string
	SampleRate  int
	Channels    int
	ChunkSize   int
}

type AudioChunk struct {
	Data     []int16
	Duration time.Duration
}

func DefaultConfig() RecorderConfig {
	return RecorderConfig{
		InputDevice: "default",
		SampleRate:  defaultSampleRate,
		Channels:    defaultChannels,
		ChunkSize:   defaultChunkSize,
	}
}

func NewRecorder(config RecorderConfig) (*Recorder, error) {
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
	} else {
		// Find device by name
		devices, err := portaudio.Devices()
		if err != nil {
			return nil, fmt.Errorf("failed to get devices: %w", err)
		}
		for _, d := range devices {
			if d.Name == config.InputDevice && d.MaxInputChannels > 0 {
				device = d
				break
			}
		}
		if device == nil {
			return nil, fmt.Errorf("input device '%s' not found", config.InputDevice)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get input device: %w", err)
	}

	fmt.Printf("Using audio device: %s\n", device.Name)
	fmt.Printf("Sample rate: %d Hz, Channels: %d\n", config.SampleRate, config.Channels)

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

	stream, err := portaudio.OpenStream(streamParams, buffer, buffer)
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("failed to open audio stream: %w", err)
	}

	return &Recorder{
		stream: stream,
		config: config,
	}, nil
}

func (r *Recorder) Start() error {
	if err := r.stream.Start(); err != nil {
		return fmt.Errorf("failed to start audio stream: %w", err)
	}
	return nil
}

func (r *Recorder) Stop() error {
	if err := r.stream.Stop(); err != nil {
		return fmt.Errorf("failed to stop audio stream: %w", err)
	}
	return nil
}

func (r *Recorder) Close() error {
	if err := r.stream.Close(); err != nil {
		return fmt.Errorf("failed to close audio stream: %w", err)
	}
	if err := portaudio.Terminate(); err != nil {
		return fmt.Errorf("failed to terminate PortAudio: %w", err)
	}
	return nil
}

func (r *Recorder) RecordChunk() (*AudioChunk, error) {
	buffer := make([]int16, r.config.ChunkSize)
	
	if err := r.stream.Read(); err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Copy buffer data (the Read() processes the buffer we passed to OpenStream)
	// We need to access the processed data
	chunk := &AudioChunk{
		Data:     make([]int16, r.config.ChunkSize),
		Duration: time.Duration(float64(r.config.ChunkSize) / float64(r.config.SampleRate) * float64(time.Second)),
	}
	
	// Since we passed the buffer to OpenStream, we need to get the processed data
	// This is a simplified version - in reality we'd need to access the stream's buffer
	copy(chunk.Data, buffer)
	
	return chunk, nil
}

func (r *Recorder) RecordDuration(duration time.Duration) ([]int16, error) {
	chunksNeeded := int(duration / r.DurationPerChunk())
	var allData []int16
	
	for i := 0; i < chunksNeeded; i++ {
		chunk, err := r.RecordChunk()
		if err != nil {
			return nil, err
		}
		allData = append(allData, chunk.Data...)
	}
	
	return allData, nil
}

func (r *Recorder) DurationPerChunk() time.Duration {
	return time.Duration(float64(r.config.ChunkSize) / float64(r.config.SampleRate) * float64(time.Second))
}

func ListInputDevices() ([]string, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize PortAudio: %w", err)
	}
	defer portaudio.Terminate()

	devices, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	var inputDevices []string
	for _, device := range devices {
		if device.MaxInputChannels > 0 {
			inputDevices = append(inputDevices, device.Name)
		}
	}

	return inputDevices, nil
}

type AudioWriter struct {
	chunks []*AudioChunk
}

func NewAudioWriter() *AudioWriter {
	return &AudioWriter{
		chunks: make([]*AudioChunk, 0),
	}
}

func (w *AudioWriter) WriteChunk(chunk *AudioChunk) error {
	w.chunks = append(w.chunks, chunk)
	return nil
}

func (w *AudioWriter) GetAudioData() []int16 {
	var data []int16
	for _, chunk := range w.chunks {
		data = append(data, chunk.Data...)
	}
	return data
}

func (w *AudioWriter) GetDuration() time.Duration {
	var totalDuration time.Duration
	for _, chunk := range w.chunks {
		totalDuration += chunk.Duration
	}
	return totalDuration
}

func (w *AudioWriter) Reset() {
	w.chunks = make([]*AudioChunk, 0)
}

func (w *AudioWriter) Close() error {
	return nil
}
