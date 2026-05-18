package audio

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// TestAudioCapture tests the audio capture functionality
func TestAudioCapture(duration time.Duration) error {
	config := DefaultConfig()
	config.ChunkSize = 1024
	
	recorder, err := NewSimpleRecorder(config)
	if err != nil {
		return fmt.Errorf("failed to create recorder: %w", err)
	}
	defer recorder.Close()

	if err := recorder.Start(); err != nil {
		return fmt.Errorf("failed to start recorder: %w", err)
	}
	defer recorder.Stop()

	fmt.Printf("Recording for %v...\n", duration)
	time.Sleep(500 * time.Millisecond) // Give user time to get ready
	
	data, err := recorder.RecordDuration(duration)
	if err != nil {
		return fmt.Errorf("failed to record: %w", err)
	}

	fmt.Printf("Recorded %d samples (%.2f seconds)\n", len(data), float64(len(data))/float64(config.SampleRate))
	
	// Calculate some statistics
	maxAmplitude := int16(0)
	for _, sample := range data {
		if sample < 0 {
			sample = -sample
		}
		if sample > maxAmplitude {
			maxAmplitude = sample
		}
	}
	fmt.Printf("Max amplitude: %d\n", maxAmplitude)
	
	return nil
}

// SaveAsWAV saves audio data as a WAV file
func SaveAsWAV(filename string, data []int16, sampleRate int, channels int) error {
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

// SaveRawAsWAV saves raw int16 audio data as a WAV file
func SaveRawAsWAV(filename string, data []int16, sampleRate int, channels int) error {
	return SaveAsWAV(filename, data, sampleRate, channels)
}

// ListDevices lists all available input devices
func ListDevices() error {
	devices, err := ListInputDevices()
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}

	fmt.Println("Available input devices:")
	for i, device := range devices {
		fmt.Printf("  %d: %s\n", i, device)
	}

	return nil
}
