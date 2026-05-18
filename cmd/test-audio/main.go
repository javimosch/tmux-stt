package main

import (
	"fmt"
	"os"
	"time"

	"github.com/javimosch/tmux-stt/internal/audio"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test-audio <list|record>")
		fmt.Println("  list    - List available audio devices")
		fmt.Println("  record  - Test audio recording (3 seconds)")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "list":
		if err := audio.ListDevices(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "record":
		duration := 3 * time.Second
		if len(os.Args) > 2 {
			var err error
			duration, err = time.ParseDuration(os.Args[2])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid duration: %v\n", err)
				os.Exit(1)
			}
		}
		
		fmt.Println("Testing audio capture...")
		if err := audio.TestAudioCapture(duration); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Audio capture test passed!")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
