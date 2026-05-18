package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/javimosch/tmux-stt/internal/audio"
	"github.com/javimosch/tmux-stt/internal/config"
	"github.com/javimosch/tmux-stt/internal/server"
	"github.com/javimosch/tmux-stt/internal/stt"
	"github.com/javimosch/tmux-stt/internal/tmux"
	"github.com/javimosch/tmux-stt/internal/translate"
	"github.com/javimosch/tmux-stt/internal/wakeword"
)

const Version = "0.1.0"

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Exit codes following semantic ranges
const (
	ExitSuccess           = 0
	ExitGenericFailure    = 1
	ExitInvalidArgument  = 80
	ExitPermissionDenied = 81
	ExitConfigError      = 82
	ExitResourceNotFound  = 92
	ExitResourceConflict  = 93
	ExitNetworkError     = 105
	ExitInternalError    = 115
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(ExitInvalidArgument)
	}

	// Global flags
	jsonFlag := flag.Bool("json", false, "Output in JSON format")
	helpJsonFlag := flag.Bool("help-json", false, "Show machine-readable help in JSON format")
	flag.Parse()

	if *helpJsonFlag {
		printHelpJson()
		os.Exit(ExitSuccess)
	}

	// Get command from non-flag arguments
	args := flag.Args()
	if len(args) < 1 {
		printHelp()
		os.Exit(ExitInvalidArgument)
	}

	command := args[0]

	switch command {
	case "start":
		handleStart(*jsonFlag)
	case "stop":
		handleStop(*jsonFlag)
	case "status":
		handleStatus(*jsonFlag)
	case "config":
		handleConfig(*jsonFlag)
	case "test":
		handleTest(*jsonFlag)
	case "tmux-list":
		handleTmuxList(*jsonFlag)
	case "pipe":
		handlePipe(*jsonFlag)
	case "version":
		handleVersion(*jsonFlag)
	case "models":
		handleModels(*jsonFlag)
	case "keywords":
		handleKeywords(*jsonFlag)
	case "engine":
		handleEngine(*jsonFlag)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printHelp()
		os.Exit(ExitInvalidArgument)
	}
}

func handleStart(jsonFlag bool) {
	startCmd := flag.NewFlagSet("start", flag.ExitOnError)
	daemon := startCmd.Bool("daemon", false, "Run as daemon")
	wakeWord := startCmd.String("wake-word", "Hey mate", "Wake word phrase")
	modelSize := startCmd.String("model", "tiny", "STT model size (tiny/base/small/medium)")
	
	// Get remaining args after global flag parsing
	args := flag.Args()
	if len(args) > 1 {
		startCmd.Parse(args[1:])
	}

	if *daemon {
		startDaemon(*wakeWord, *modelSize, jsonFlag)
	} else {
		startForeground(*wakeWord, *modelSize, jsonFlag)
	}
}

func handleStop(jsonFlag bool) {
	stopDaemon(jsonFlag)
}

func handleStatus(jsonFlag bool) {
	checkDaemonStatus(jsonFlag)
}

func handleConfig(jsonFlag bool) {
	configCmd := flag.NewFlagSet("config", flag.ExitOnError)
	set := configCmd.String("set", "", "Configuration key=value (e.g., wake-word=\"Hey mate\")")
	get := configCmd.String("get", "", "Configuration key to get")
	list := configCmd.Bool("list", false, "List all configuration")
	
	// Get remaining args after global flag parsing
	args := flag.Args()
	if len(args) > 1 {
		configCmd.Parse(args[1:])
	}

	cfg, err := config.Load()
	if err != nil {
		outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		return
	}

	if *list {
		listConfig(cfg, jsonFlag)
	} else if *set != "" {
		setConfig(cfg, *set, jsonFlag)
	} else if *get != "" {
		getConfig(cfg, *get, jsonFlag)
	} else {
		fmt.Fprintf(os.Stderr, "Use --set, --get, or --list\n")
		os.Exit(ExitInvalidArgument)
	}
}

func handleTest(jsonFlag bool) {
	testCmd := flag.NewFlagSet("test", flag.ExitOnError)
	transcribeOnly := testCmd.Bool("transcribe-only", false, "Test transcription without wake word")
	wakeWordTest := testCmd.Bool("wake-word", false, "Test wake word detection only")
	translationTest := testCmd.Bool("translation", false, "Test translation functionality")
	tmuxTest := testCmd.Bool("tmux", false, "Test tmux integration")
	
	// Get remaining args after global flag parsing
	args := flag.Args()
	if len(args) > 1 {
		testCmd.Parse(args[1:])
	}

	if *transcribeOnly {
		testTranscription(jsonFlag)
	} else if *wakeWordTest {
		testWakeWord(jsonFlag)
	} else if *translationTest {
		testTranslation(jsonFlag)
	} else if *tmuxTest {
		testTmux(jsonFlag)
	} else {
		fmt.Fprintf(os.Stderr, "Use --transcribe-only, --wake-word, --translation, or --tmux\n")
		os.Exit(ExitInvalidArgument)
	}
}

func handleTmuxList(jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	// Create tmux client
	tmuxClient := tmux.NewTmuxClient(cfg.Tmux.SocketPath, cfg.Tmux.AutoEnter)
	
	// List sessions
	sessions, err := tmuxClient.ListSessions()
	if err != nil {
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    100,
					"type":    "external_error",
					"message": "Failed to list tmux sessions",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": true,
				},
			}
			outputJSON(output)
		} else {
			fmt.Printf("❌ Failed to list sessions: %v\n", err)
			fmt.Println("Make sure tmux is running")
		}
		return
	}
	
	if jsonFlag {
		output := map[string]interface{}{
			"sessions": sessions,
		}
		
		// Try to get current session info
		if tmuxClient.IsInsideTmux() {
			currentSession, _ := tmuxClient.GetCurrentSession()
			currentWindow, _ := tmuxClient.GetCurrentWindow()
			currentPane, _ := tmuxClient.GetCurrentPane()
			
			output["current"] = map[string]interface{}{
				"session": currentSession,
				"window": currentWindow,
				"pane": currentPane,
			}
			
			// List windows in current session
			windows, err := tmuxClient.ListWindows()
			if err == nil {
				output["windows"] = windows
			}
			
			// List panes in current window
			panes, err := tmuxClient.ListPanes()
			if err == nil {
				output["panes"] = panes
			}
		}
		
		outputJSON(output)
	} else {
		fmt.Println("🖥️  Listing tmux sessions, windows, and panes...")
		fmt.Println()
		
		if len(sessions) == 0 {
			fmt.Println("❌ No tmux sessions found")
			fmt.Println("Start a tmux session with: tmux new -s mysession")
			return
		}
		
		fmt.Println("📋 Available tmux sessions:")
		for i, session := range sessions {
			fmt.Printf("  %d. %s\n", i+1, session)
		}
		fmt.Println()
		
		// Try to get current session info
		if tmuxClient.IsInsideTmux() {
			currentSession, _ := tmuxClient.GetCurrentSession()
			currentWindow, _ := tmuxClient.GetCurrentWindow()
			currentPane, _ := tmuxClient.GetCurrentPane()
			
			fmt.Println("📍 Current location:")
			fmt.Printf("  Session: %s\n", currentSession)
			fmt.Printf("  Window: %s\n", currentWindow)
			fmt.Printf("  Pane: %s\n", currentPane)
			fmt.Println()
			
			// List windows in current session
			windows, err := tmuxClient.ListWindows()
			if err == nil {
				fmt.Println("📋 Windows in current session:")
				for i, window := range windows {
					fmt.Printf("  %d. %s\n", i+1, window)
				}
				fmt.Println()
			}
			
			// List panes in current window
			panes, err := tmuxClient.ListPanes()
			if err == nil {
				fmt.Println("📋 Panes in current window:")
				for i, pane := range panes {
					fmt.Printf("  %d. %s\n", i+1, pane)
				}
				fmt.Println()
			}
			
			fmt.Println("💡 To target a specific pane:")
			fmt.Println("  tmux-stt config --set tmux.target-pane=session:window.pane")
			fmt.Println("  Example: tmux-stt config --set tmux.target-pane=mysession:0.0")
			fmt.Println()
			fmt.Println("💡 To target a specific window:")
			fmt.Println("  tmux-stt config --set tmux.target-window=session:window")
			fmt.Println("  Example: tmux-stt config --set tmux.target-window=mysession:0")
		} else {
			fmt.Println("ℹ️  Not running inside tmux")
			fmt.Println("Run this command inside a tmux session for detailed information")
		}
	}
}

func handlePipe(jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if jsonFlag {
		fmt.Println("JSON output not supported for pipe mode")
		os.Exit(ExitInvalidArgument)
	}

	pipeCmd := flag.NewFlagSet("pipe", flag.ExitOnError)
	wakeWord := pipeCmd.String("wake-word", cfg.WakeWord, "Wake word phrase")
	modelSize := pipeCmd.String("model", cfg.STT.ModelSize, "STT model size")
	logFile := pipeCmd.String("log-file", "/var/log/tmux-stt.log", "Log file for debugging")
	
	// Get remaining args after global flag parsing
	args := flag.Args()
	if len(args) > 1 {
		pipeCmd.Parse(args[1:])
	}

	// Override config with command line args
	if *wakeWord != cfg.WakeWord {
		cfg.WakeWord = *wakeWord
	}
	if *modelSize != cfg.STT.ModelSize {
		cfg.STT.ModelSize = *modelSize
	}

	// Open log file for debugging
	var logFileHandle *os.File
	if *logFile != "" {
		// Create log directory if it doesn't exist
		logDir := "/var/log"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// If we can't create /var/log, use current directory
			*logFile = "tmux-stt.log"
		}
		
		logFileHandle, err = os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// If logging fails, continue without logging
			logFileHandle = nil
		} else {
			// Redirect stderr to log file for debugging
			os.Stderr = logFileHandle
			logFileHandle.WriteString("=== tmux-stt pipe mode started ===\n")
			logFileHandle.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
			logFileHandle.WriteString(fmt.Sprintf("Wake word: '%s'\n", cfg.WakeWord))
			logFileHandle.WriteString(fmt.Sprintf("Translation: %v\n", cfg.Translation.Enabled))
			logFileHandle.WriteString(fmt.Sprintf("Target language: %s\n", cfg.Translation.TargetLang))
			logFileHandle.WriteString("\n")
		}
	}

	// Redirect stdout to /dev/null for clean pipe output
	devNull, _ := os.OpenFile("/dev/null", os.O_WRONLY, 0644)
	os.Stdout = devNull

	// Save original stdout for final transcription output
	originalStdout := os.NewFile(uintptr(1), "/dev/stdout")

	// Create audio recorder (quiet mode for pipe)
	audioConfig := audio.RecorderConfig{
		InputDevice: cfg.Audio.InputDevice,
		SampleRate:  cfg.Audio.SampleRate,
		Channels:    cfg.Audio.Channels,
		ChunkSize:   cfg.Audio.ChunkSize,
	}
	recorder, err := audio.NewSimpleRecorderQuiet(audioConfig, true)
	if err != nil {
		os.Exit(1)
	}
	defer recorder.Close()

	// Initialize STT engine with language support
	sttEngine, err := stt.NewWhisperBinaryWithLanguage(cfg.ModelsDir, cfg.STT.ModelSize, cfg.STT.Language)
	if err != nil {
		os.Exit(1)
	}

	// Initialize translator
	translator := translate.NewOpenAITranslator(
		cfg.Translation.APIKey,
		cfg.Translation.APIEndpoint,
		cfg.Translation.Model,
	)

	// Create STT adapter (quiet mode for pipe)
	sttAdapter := wakeword.NewWhisperBinaryAdapterQuiet(sttEngine)

	// Create VAD using factory based on configuration
	vad, err := wakeword.VADFactory(cfg)
	if err != nil {
		if logFileHandle != nil {
			logFileHandle.WriteString(fmt.Sprintf("Failed to initialize VAD: %v\n", err))
		}
		os.Exit(1)
	}
	defer vad.Close()

	// Initialize wake word detector with VAD interface
	wakeWordConfig := wakeword.WakeWordConfig{
		WakeWord:    cfg.WakeWord,
		SampleRate:  cfg.Audio.SampleRate,
		ChunkSize:   cfg.Audio.ChunkSize,
		SilenceMs:   cfg.VAD.SilenceMs,
		MinSpeechMs: cfg.VAD.SpeechMs,
		Threshold:   int16(cfg.VAD.Threshold),
	}
	wakeDetector, err := wakeword.NewWakeWordDetector(wakeWordConfig, sttAdapter, vad)
	if err != nil {
		if logFileHandle != nil {
			logFileHandle.WriteString(fmt.Sprintf("Failed to initialize wake word detector: %v\n", err))
		}
		os.Exit(1)
	}

	// Start audio recording
	if err := recorder.Start(); err != nil {
		if logFileHandle != nil {
			logFileHandle.WriteString(fmt.Sprintf("Failed to start audio: %v\n", err))
		}
		os.Exit(1)
	}
	defer recorder.Stop()

	// Close log file on exit
	if logFileHandle != nil {
		defer func() {
			logFileHandle.WriteString("=== tmux-stt pipe mode stopped ===\n")
			logFileHandle.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
			logFileHandle.Close()
		}()
	}

	// Main listening loop
	for {
		// Record until silence is detected (max 20 seconds)
		silenceDuration := time.Duration(cfg.VAD.SilenceMs) * time.Millisecond
		minSilenceChunks := max(1, int(silenceDuration.Milliseconds())*cfg.Audio.SampleRate/(cfg.Audio.ChunkSize*1000)/2) // At least 1 chunk, roughly half the duration
		
		audioData, err := recorder.RecordUntilSilence(20*time.Second, int16(cfg.VAD.Threshold), minSilenceChunks)
		if err != nil {
			if logFileHandle != nil {
				logFileHandle.WriteString(fmt.Sprintf("Error recording audio: %v\n", err))
			}
			time.Sleep(1 * time.Second)
			continue
		}

		// Skip if no speech was detected (empty or very short recording)
		minSpeechSamples := cfg.Audio.SampleRate * cfg.VAD.SpeechMs / 1000
		if len(audioData) < minSpeechSamples {
			if logFileHandle != nil {
				logFileHandle.WriteString("No speech detected, listening again...\n")
			}
			continue
		}

		// Detect wake word in recorded audio
		detected, transcription, err := wakeDetector.DetectWakeWordInAudio(audioData)
		if err != nil {
			if logFileHandle != nil {
				logFileHandle.WriteString(fmt.Sprintf("Error detecting wake word: %v\n", err))
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if !detected {
			if logFileHandle != nil {
				logFileHandle.WriteString("Wake word not detected\n")
			}
			continue
		}

		if logFileHandle != nil {
			logFileHandle.WriteString(fmt.Sprintf("✅ Wake word detected! Transcription: '%s'\n", transcription))
		}

		// Strip wake word from transcription if configured
		if cfg.StripWakeWord {
			transcription = wakeword.StripWakeWord(transcription, cfg.WakeWord)
			if transcription != "" {
				if logFileHandle != nil {
					logFileHandle.WriteString(fmt.Sprintf("🔪 Stripped wake word: '%s'\n", transcription))
				}
			}
		}

		// Apply translation if enabled
		if cfg.Translation.Enabled && cfg.Translation.APIKey != "" {
			if logFileHandle != nil {
				logFileHandle.WriteString(fmt.Sprintf("🌐 Translating to %s...\n", cfg.Translation.TargetLang))
			}
			
			translated, err := translator.Translate(transcription, cfg.Translation.TargetLang)
			if err != nil {
				if logFileHandle != nil {
					logFileHandle.WriteString(fmt.Sprintf("❌ Translation error: %v\n", err))
					logFileHandle.WriteString(fmt.Sprintf("📝 Using original text: '%s'\n", transcription))
				}
			} else {
				if logFileHandle != nil {
					logFileHandle.WriteString(fmt.Sprintf("📝 Translated: '%s'\n", translated))
				}
				transcription = translated
			}
		}

		// Output to original stdout (for piping) - ONLY this output
		originalStdout.WriteString(transcription + "\n")

		if logFileHandle != nil {
			logFileHandle.WriteString(fmt.Sprintf("📤 Sent to pipe: '%s'\n", transcription))
			logFileHandle.WriteString("---\n")
		}

		// Brief pause before listening again
		time.Sleep(500 * time.Millisecond)
	}
}

func handleVersion(jsonFlag bool) {
	if jsonFlag {
		output := map[string]interface{}{
			"version": Version,
			"name":    "tmux-stt",
			"status":  "success",
		}
		outputJSON(output)
	} else {
		fmt.Printf("tmux-stt v%s\n", Version)
	}
}

func printHelpJson() {
	help := map[string]interface{}{
		"version": Version,
		"name":    "tmux-stt",
		"commands": map[string]interface{}{
			"start": map[string]interface{}{
				"description": "Start voice recognition daemon",
				"flags": map[string]string{
					"--daemon":    "Run as daemon (background)",
					"--wake-word": "Wake word phrase",
					"--model":     "STT model size (tiny/base/small/medium)",
					"--json":      "Output in JSON format",
				},
			},
			"stop": map[string]interface{}{
				"description": "Stop voice recognition daemon",
				"flags": map[string]string{
					"--json": "Output in JSON format",
				},
			},
			"status": map[string]interface{}{
				"description": "Check daemon status",
				"flags": map[string]string{
					"--json": "Output in JSON format",
				},
			},
			"config": map[string]interface{}{
				"description": "Manage configuration",
				"flags": map[string]string{
					"--set":  "Configuration key=value",
					"--get":  "Configuration key to get",
					"--list": "List all configuration",
					"--json": "Output in JSON format",
				},
			},
			"test": map[string]interface{}{
				"description": "Test components",
				"flags": map[string]string{
					"--transcribe-only": "Test transcription without wake word",
					"--wake-word":       "Test wake word detection only",
					"--translation":     "Test translation functionality",
					"--tmux":            "Test tmux integration",
					"--json":            "Output in JSON format",
				},
			},
			"tmux-list": map[string]interface{}{
				"description": "List tmux sessions, windows, panes",
				"flags": map[string]string{
					"--json": "Output in JSON format",
				},
			},
			"pipe": map[string]interface{}{
				"description": "Output transcriptions to stdout (for piping)",
				"flags": map[string]string{
					"--wake-word": "Wake word phrase",
					"--model":     "STT model size",
					"--log-file":  "Path to log file for debugging",
					"--json":      "Output in JSON format",
				},
			},
			"version": map[string]interface{}{
				"description": "Show version information",
				"flags": map[string]string{
					"--json": "Output in JSON format",
				},
			},
		},
		"output_formats": []string{"text", "json"},
		"exit_codes": map[int]string{
			0:   "success",
			80:  "invalid_argument",
			82:  "config_error",
			92:  "resource_not_found",
			105: "network_error",
			115: "internal_error",
		},
	}
	outputJSON(help)
}

func outputJSON(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(ExitInternalError)
	}
	fmt.Println(string(jsonData))
}

func outputError(jsonFlag bool, message string, exitCode int, err error) {
	if jsonFlag {
		errorOutput := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    exitCode,
				"type":    getErrorType(exitCode),
				"message": message,
				"details": map[string]interface{}{
					"original_error": err.Error(),
				},
				"recoverable": exitCode >= 100 && exitCode < 110,
			},
		}
		outputJSON(errorOutput)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	}
	os.Exit(exitCode)
}

func getErrorType(exitCode int) string {
	switch {
	case exitCode >= 80 && exitCode < 90:
		return "user_error"
	case exitCode >= 90 && exitCode < 100:
		return "resource_error"
	case exitCode >= 100 && exitCode < 110:
		return "external_error"
	case exitCode >= 110 && exitCode < 120:
		return "internal_error"
	default:
		return "unknown_error"
	}
}

func printHelp() {
	fmt.Println("tmux-stt - Voice wake word + STT + tmux paste daemon")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  tmux-stt <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start       Start voice recognition daemon")
	fmt.Println("  stop        Stop voice recognition daemon")
	fmt.Println("  status      Check daemon status")
	fmt.Println("  config      Manage configuration")
	fmt.Println("  test        Test components")
	fmt.Println("  tmux-list   List tmux sessions, windows, panes")
	fmt.Println("  pipe        Output transcriptions to stdout (for piping)")
	fmt.Println("  version     Show version information")
	fmt.Println()
	fmt.Println("Pipe Options:")
	fmt.Println("  -log-file        Path to log file (default: /var/log/tmux-stt.log)")
	fmt.Println("  -wake-word       Wake word phrase")
	fmt.Println("  -model           STT model size")
	fmt.Println()
	fmt.Println("Start Options:")
	fmt.Println("  -daemon         Run as daemon (background)")
	fmt.Println("  -wake-word      Wake word phrase (default: \"Hey mate\")")
	fmt.Println("  -model          STT model size (default: tiny)")
	fmt.Println()
	fmt.Println("Config Options:")
	fmt.Println("  -set key=value    Set configuration")
	fmt.Println("  -get key          Get configuration")
	fmt.Println("  -list             List all configuration")
	fmt.Println()
	fmt.Println("Test Options:")
	fmt.Println("  -transcribe-only    Test transcription without wake word")
	fmt.Println("  -wake-word          Test wake word detection only")
	fmt.Println("  -translation        Test translation functionality")
	fmt.Println("  -tmux               Test tmux integration")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  tmux-stt start")
	fmt.Println("  tmux-stt start -daemon -wake-word \"Hey Jarvis\"")
	fmt.Println("  tmux-stt start -daemon -model base")
	fmt.Println("  tmux-stt config --list")
	fmt.Println("  tmux-stt config --set wake-word=\"Computer\"")
	fmt.Println("  tmux-stt test --transcribe-only")
	fmt.Println("  tmux-stt pipe | devin")
	fmt.Println("  tmux-stt pipe -log-file /var/log/tmux-stt.log | devin")
	fmt.Println("  tmux-stt stop")
	fmt.Println("  tmux-stt status")
}

func startDaemon(wakeWord, modelSize string, jsonFlag bool) {
	startDaemonProcess(wakeWord, modelSize, jsonFlag)
}

func startForeground(wakeWord, modelSize string, jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	// Override with command line args
	if wakeWord != "Hey mate" {
		cfg.WakeWord = wakeWord
	}
	if modelSize != "tiny" {
		cfg.STT.ModelSize = modelSize
	}

	// Get whisper binary path
	whisperBinary := "/tmp/whisper.cpp-local"
	
	// Create server
	srv, err := server.NewServer(cfg, whisperBinary)
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to create server", ExitInternalError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		}
		os.Exit(ExitInternalError)
	}

	// Handle graceful shutdown
	defer func() {
		if err := srv.Stop(); err != nil {
			if jsonFlag {
				outputError(jsonFlag, "Error stopping server", ExitInternalError, err)
			} else {
				fmt.Fprintf(os.Stderr, "Error stopping server: %v\n", err)
			}
		}
	}()

	// Start server (blocking)
	if err := srv.Start(); err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Server error", ExitInternalError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
		os.Exit(ExitInternalError)
	}
}



func listConfig(cfg *config.Config, jsonFlag bool) {
	if jsonFlag {
		output := map[string]interface{}{
			"config": map[string]interface{}{
				"wake_word": cfg.WakeWord,
				"strip_wake_word": cfg.StripWakeWord,
				"stt": map[string]interface{}{
					"language": cfg.STT.Language,
					"model_size": cfg.STT.ModelSize,
				},
				"vad": map[string]interface{}{
					"silence_ms": cfg.VAD.SilenceMs,
					"speech_ms": cfg.VAD.SpeechMs,
					"threshold": cfg.VAD.Threshold,
				},
				"translation": map[string]interface{}{
					"enabled": cfg.Translation.Enabled,
					"source_lang": cfg.Translation.SourceLang,
					"target_lang": cfg.Translation.TargetLang,
					"api_endpoint": cfg.Translation.APIEndpoint,
				},
				"tmux": map[string]interface{}{
					"socket_path": cfg.Tmux.SocketPath,
					"auto_enter": cfg.Tmux.AutoEnter,
				},
				"audio": map[string]interface{}{
					"input_device": cfg.Audio.InputDevice,
					"sample_rate": cfg.Audio.SampleRate,
					"channels": cfg.Audio.Channels,
					"silence_threshold": cfg.Audio.SilenceThreshold,
					"silence_duration_ms": cfg.Audio.SilenceDurationMs,
					"min_speech_duration_ms": cfg.Audio.MinSpeechDurationMs,
				},
				"models_dir": cfg.ModelsDir,
			},
		}
		outputJSON(output)
	} else {
		fmt.Println("Current configuration:")
		fmt.Printf("  Wake word: %s\n", cfg.WakeWord)
		fmt.Printf("  Wake word method: %s\n", cfg.WakeWordMethod)
		fmt.Printf("  Strip wake word from output: %v\n", cfg.StripWakeWord)
		fmt.Printf("  STT Language: %s\n", cfg.STT.Language)
		fmt.Printf("  STT Model size: %s\n", cfg.STT.ModelSize)
		fmt.Printf("  VAD Method: %s\n", cfg.VAD.Method)
		fmt.Printf("  VAD Silence duration: %d ms\n", cfg.VAD.SilenceMs)
		fmt.Printf("  VAD Min speech duration: %d ms\n", cfg.VAD.SpeechMs)
		fmt.Printf("  VAD Threshold: %d\n", cfg.VAD.Threshold)
		fmt.Printf("  Translation enabled: %v\n", cfg.Translation.Enabled)
		fmt.Printf("  Translation source: %s\n", cfg.Translation.SourceLang)
		fmt.Printf("  Translation target: %s\n", cfg.Translation.TargetLang)
		fmt.Printf("  Translation endpoint: %s\n", cfg.Translation.APIEndpoint)
		fmt.Printf("  Tmux socket: %s\n", cfg.Tmux.SocketPath)
		fmt.Printf("  Tmux auto-enter: %v\n", cfg.Tmux.AutoEnter)
		fmt.Printf("  Audio device: %s\n", cfg.Audio.InputDevice)
		fmt.Printf("  Audio sample rate: %d Hz\n", cfg.Audio.SampleRate)
		fmt.Printf("  Audio channels: %d\n", cfg.Audio.Channels)
		fmt.Printf("  Silence threshold: %d\n", cfg.Audio.SilenceThreshold)
		fmt.Printf("  Silence duration: %d ms\n", cfg.Audio.SilenceDurationMs)
		fmt.Printf("  Min speech duration: %d ms\n", cfg.Audio.MinSpeechDurationMs)
		fmt.Printf("  Models directory: %s\n", cfg.ModelsDir)
	}
}

func setConfig(cfg *config.Config, keyValue string, jsonFlag bool) {
	parts := strings.SplitN(keyValue, "=", 2)
	if len(parts) != 2 {
		err := fmt.Errorf("invalid format. Use: key=value")
		if jsonFlag {
			outputError(jsonFlag, "Invalid format. Use: key=value", ExitInvalidArgument, err)
		} else {
			fmt.Fprintf(os.Stderr, "Invalid format. Use: key=value\n")
		}
		os.Exit(ExitInvalidArgument)
	}

	key, value := parts[0], parts[1]
	if err := cfg.Set(key, value); err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to set config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to set config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if err := config.Save(cfg); err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to save config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if jsonFlag {
		output := map[string]interface{}{
			"status": "set",
			"key": key,
			"value": value,
			"message": "Configuration updated successfully",
		}
		outputJSON(output)
	} else {
		fmt.Printf("Set %s = %s\n", key, value)
	}
}

func getConfig(cfg *config.Config, key string, jsonFlag bool) {
	value, err := cfg.Get(key)
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to get config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to get config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if jsonFlag {
		output := map[string]interface{}{
			"key": key,
			"value": value,
		}
		outputJSON(output)
	} else {
		fmt.Printf("%s = %s\n", key, value)
	}
}

func testTranscription(jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if jsonFlag {
		fmt.Println("JSON output not supported for interactive test mode")
		os.Exit(ExitInvalidArgument)
	}

	fmt.Println("🎙️  Testing transcription with audio capture...")
	fmt.Println()

	// Try to create whisper engine to check if it's available
	_, err = stt.NewWhisperBinary(cfg.ModelsDir, cfg.STT.ModelSize)
	if err != nil {
		fmt.Println("❌ whisper.cpp not found")
		fmt.Println()
		fmt.Println("Error:", err.Error())
		fmt.Println()
		fmt.Println("For now, testing audio capture only...")
		testAudioCaptureOnly(cfg)
		return
	}

	fmt.Println("✅ whisper.cpp found")
	fmt.Println("Proceeding with full transcription test...")
	fmt.Println()

	// Test audio capture + transcription
	testFullTranscription(cfg)
}

func testAudioCaptureOnly(cfg *config.Config) {
	// Configure audio recorder
	audioConfig := audio.RecorderConfig{
		InputDevice: cfg.Audio.InputDevice,
		SampleRate:  cfg.Audio.SampleRate,
		Channels:    cfg.Audio.Channels,
		ChunkSize:   cfg.Audio.ChunkSize,
	}

	fmt.Printf("Audio configuration: %d Hz, %d channels, chunk size %d\n", 
		audioConfig.SampleRate, audioConfig.Channels, audioConfig.ChunkSize)

	// Create recorder
	recorder, err := audio.NewSimpleRecorder(audioConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create recorder: %v\n", err)
		os.Exit(1)
	}
	defer recorder.Close()

	// Start recording
	if err := recorder.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start recorder: %v\n", err)
		os.Exit(1)
	}
	defer recorder.Stop()

	// Record for 3 seconds
	duration := 3 * time.Second
	fmt.Printf("Recording for %v...\n", duration)
	fmt.Println("Speak now!")
	
	time.Sleep(1 * time.Second) // Give user time to prepare
	
	data, err := recorder.RecordDuration(duration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to record: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Recording complete: %d samples (%.2f seconds)\n", len(data), float64(len(data))/float64(cfg.Audio.SampleRate))
	
	// Save as WAV file
	wavFile := "/tmp/test-recording.wav"
	if err := audio.SaveRawAsWAV(wavFile, data, cfg.Audio.SampleRate, cfg.Audio.Channels); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save WAV file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✅ Audio saved to: %s\n", wavFile)
	fmt.Println()
	fmt.Println("🎉 Audio capture test passed!")
	fmt.Println("📝 Install whisper.cpp to enable transcription functionality")
}

func testFullTranscription(cfg *config.Config) {
	// Configure audio recorder
	audioConfig := audio.RecorderConfig{
		InputDevice: cfg.Audio.InputDevice,
		SampleRate:  cfg.Audio.SampleRate,
		Channels:    cfg.Audio.Channels,
		ChunkSize:   cfg.Audio.ChunkSize,
	}

	// Create recorder
	recorder, err := audio.NewSimpleRecorder(audioConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create recorder: %v\n", err)
		os.Exit(1)
	}
	defer recorder.Close()

	// Start recording
	if err := recorder.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start recorder: %v\n", err)
		os.Exit(1)
	}
	defer recorder.Stop()

	// Record for 5 seconds
	duration := 5 * time.Second
	fmt.Printf("Recording for %v...\n", duration)
	fmt.Println("Speak now!")
	
	time.Sleep(1 * time.Second) // Give user time to prepare
	
	data, err := recorder.RecordDuration(duration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to record: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Recording complete: %d samples (%.2f seconds)\n", len(data), float64(len(data))/float64(cfg.Audio.SampleRate))
	
	// Save as WAV file
	wavFile := "/tmp/test-recording.wav"
	if err := audio.SaveRawAsWAV(wavFile, data, cfg.Audio.SampleRate, cfg.Audio.Channels); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save WAV file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✅ Audio saved to: %s\n", wavFile)

	// Transcribe using whisper.cpp
	fmt.Println("🎯 Transcribing audio...")
	
	whisper, err := stt.NewWhisperBinary(cfg.ModelsDir, cfg.STT.ModelSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create whisper engine: %v\n", err)
		os.Exit(1)
	}

	result, err := whisper.TranscribeFile(wavFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to transcribe: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("📝 Transcription Result:")
	fmt.Println("-------------------")
	fmt.Println(result.Text)
	fmt.Println("-------------------")
	fmt.Println()
	fmt.Println("🎉 Full transcription test passed!")
}

func testWakeWord(jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if jsonFlag {
		fmt.Println("JSON output not supported for interactive test mode")
		os.Exit(ExitInvalidArgument)
	}

	fmt.Println("🎤 Testing wake word detection...")
	fmt.Println()
	fmt.Printf("Wake word: '%s'\n", cfg.WakeWord)
	fmt.Println()

	// Try to create STT engine
	whisper, err := stt.NewWhisperBinary(cfg.ModelsDir, cfg.STT.ModelSize)
	if err != nil {
		fmt.Println("❌ STT engine not available")
		fmt.Println("Error:", err.Error())
		fmt.Println()
		fmt.Println("Wake word detection requires STT for transcription")
		fmt.Println("Please install whisper.cpp first")
		return
	}

	// Create VAD using factory based on configuration
	vad, err := wakeword.VADFactory(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize VAD: %v\n", err)
		return
	}
	defer vad.Close()

	// Create wake word detector
	wakeWordConfig := wakeword.WakeWordConfig{
		WakeWord:    cfg.WakeWord,
		SampleRate:  cfg.Audio.SampleRate,
		ChunkSize:   cfg.Audio.ChunkSize,
		SilenceMs:   1000,
		MinSpeechMs: 500,
		Threshold:   500,
	}

	adapter := wakeword.NewWhisperBinaryAdapter(whisper)
	detector, err := wakeword.NewWakeWordDetector(wakeWordConfig, adapter, vad)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create wake word detector: %v\n", err)
		os.Exit(1)
	}

	// Set up callback to show detection results
	detector.SetDetectionCallback(func(detected bool, transcription string) {
		if detected {
			fmt.Println("🎯 WAKE WORD DETECTED!")
			fmt.Printf("Transcription: '%s'\n", transcription)
		}
	})

	// Configure audio recorder
	audioConfig := audio.RecorderConfig{
		InputDevice: cfg.Audio.InputDevice,
		SampleRate:  cfg.Audio.SampleRate,
		Channels:    cfg.Audio.Channels,
		ChunkSize:   cfg.Audio.ChunkSize,
	}

	// Create recorder
	recorder, err := audio.NewSimpleRecorder(audioConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create recorder: %v\n", err)
		os.Exit(1)
	}
	defer recorder.Close()

	// Start recording
	if err := recorder.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start recorder: %v\n", err)
		os.Exit(1)
	}
	defer recorder.Stop()

	// Record for 10 seconds to test wake word detection
	duration := 10 * time.Second
	fmt.Printf("Listening for wake word '%s' for %v...\n", cfg.WakeWord, duration)
	fmt.Println("Speak now!")
	
	time.Sleep(1 * time.Second) // Give user time to prepare
	
	data, err := recorder.RecordDuration(duration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to record: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Recording complete: %d samples (%.2f seconds)\n", len(data), float64(len(data))/float64(cfg.Audio.SampleRate))
	
	// Save as WAV file for reference
	wavFile := "/tmp/wake-word-test.wav"
	if err := audio.SaveRawAsWAV(wavFile, data, cfg.Audio.SampleRate, cfg.Audio.Channels); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save WAV file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Audio saved to: %s\n", wavFile)

	// Test wake word detection in the recorded audio
	fmt.Println()
	fmt.Println("🎯 Analyzing audio for wake word...")
	
	detected, transcription, err := detector.DetectWakeWordInAudio(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during wake word detection: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	if detected {
		fmt.Println("✅ WAKE WORD DETECTED!")
		fmt.Printf("Transcription: '%s'\n", transcription)
		fmt.Println()
		fmt.Println("🎉 Wake word detection test passed!")
	} else {
		fmt.Println("❌ Wake word NOT detected")
		fmt.Printf("Transcription: '%s'\n", transcription)
		fmt.Println()
		fmt.Println("💡 Try speaking more clearly or get closer to the microphone")
		fmt.Println("💡 You can also adjust the wake word in config")
	}
}

func testTranslation(jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if jsonFlag {
		fmt.Println("JSON output not supported for interactive test mode")
		os.Exit(ExitInvalidArgument)
	}

	fmt.Println("🌐 Testing translation functionality...")
	fmt.Println()
	
	if !cfg.Translation.Enabled {
		fmt.Println("❌ Translation is disabled in config")
		fmt.Println("Enable it with: tmux-stt config --set translation.enabled=true")
		fmt.Println()
		fmt.Println("Testing with translation disabled (will return original text)...")
	}
	
	if cfg.Translation.APIKey == "" {
		fmt.Println("⚠️  No API key configured")
		fmt.Println("Set it with: tmux-stt config --set translation.api-key=YOUR_KEY")
		fmt.Println()
		fmt.Println("Testing without API key (will return original text)...")
	}

	fmt.Printf("Translation endpoint: %s\n", cfg.Translation.APIEndpoint)
	fmt.Printf("Target language: %s\n", cfg.Translation.TargetLang)
	fmt.Println()

	// Create translator
	translator := translate.NewOpenAITranslator(
		cfg.Translation.APIKey,
		cfg.Translation.APIEndpoint,
		cfg.Translation.Model,
	)

	// Test with sample text (Spanish to English)
	testText := "Crea una función que calcule el factorial de un número"
	fmt.Printf("Original text (Spanish): '%s'\n", testText)
	
	translated, err := translator.Translate(testText, cfg.Translation.TargetLang)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Translation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Translated text: '%s'\n", translated)
	fmt.Println()
	
	if translated == testText {
		fmt.Println("ℹ️  Translation returned original text (likely disabled or no API key)")
	} else {
		fmt.Println("✅ Translation test passed!")
	}
}

func testTmux(jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		if jsonFlag {
			outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		}
		os.Exit(ExitConfigError)
	}

	if jsonFlag {
		fmt.Println("JSON output not supported for interactive test mode")
		os.Exit(ExitInvalidArgument)
	}

	fmt.Println("🖥️  Testing tmux integration...")
	fmt.Println()
	
	// Create tmux client
	tmuxClient := tmux.NewTmuxClient(cfg.Tmux.SocketPath, cfg.Tmux.AutoEnter)
	
	// Check if inside tmux
	if !tmuxClient.IsInsideTmux() {
		fmt.Println("❌ Not running inside tmux")
		fmt.Println("This test must be run inside a tmux session")
		fmt.Println()
		fmt.Println("To start a tmux session:")
		fmt.Println("  tmux new -s test")
		fmt.Println()
		fmt.Println("Then run this command inside the tmux session")
		return
	}
	
	fmt.Println("✅ Running inside tmux")
	
	// Get current session info
	session, err := tmuxClient.GetCurrentSession()
	if err != nil {
		fmt.Printf("⚠️  Could not get session: %v\n", err)
	} else {
		fmt.Printf("Session: %s\n", session)
	}
	
	window, err := tmuxClient.GetCurrentWindow()
	if err != nil {
		fmt.Printf("⚠️  Could not get window: %v\n", err)
	} else {
		fmt.Printf("Window: %s\n", window)
	}
	
	pane, err := tmuxClient.GetCurrentPane()
	if err != nil {
		fmt.Printf("⚠️  Could not get pane: %v\n", err)
	} else {
		fmt.Printf("Pane: %s\n", pane)
	}
	
	fmt.Println()
	fmt.Println("Testing tmux send-keys...")
	
	testText := "echo 'Hello from tmux-stt!'"
	fmt.Printf("Sending: '%s'\n", testText)
	
	if err := tmuxClient.SendKeys(testText); err != nil {
		fmt.Printf("❌ Failed to send keys: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("✅ Successfully sent keys to tmux")
	fmt.Println("Check your tmux session for the echo command output")
	fmt.Println()
	fmt.Println("🎉 tmux integration test passed!")
}

func handleModels(jsonFlag bool) {
	modelsCmd := flag.NewFlagSet("models", flag.ExitOnError)
	download := modelsCmd.String("download", "", "Download model (vad-silero, kws)")
	list := modelsCmd.Bool("list", false, "List available models")
	status := modelsCmd.Bool("status", false, "Show model status")
	
	args := flag.Args()
	if len(args) > 1 {
		modelsCmd.Parse(args[1:])
	}

	if *download != "" {
		downloadModel(*download, jsonFlag)
	} else if *list {
		listModels(jsonFlag)
	} else if *status {
		modelStatus(jsonFlag)
	} else {
		if jsonFlag {
			outputJSON(map[string]string{
				"error": "Use --download, --list, or --status",
			})
		} else {
			fmt.Println("Model management commands:")
			fmt.Println("  tmux-stt models --download vad-silero  # Download Silero VAD model")
			fmt.Println("  tmux-stt models --download kws         # Download KWS models")
			fmt.Println("  tmux-stt models --list                # List available models")
			fmt.Println("  tmux-stt models --status              # Show model status")
		}
	}
}

func handleKeywords(jsonFlag bool) {
	keywordsCmd := flag.NewFlagSet("keywords", flag.ExitOnError)
	set := keywordsCmd.String("set", "", "Set keywords (comma-separated)")
	list := keywordsCmd.Bool("list", false, "List current keywords")
	clear := keywordsCmd.Bool("clear", false, "Clear keywords")
	
	args := flag.Args()
	if len(args) > 1 {
		keywordsCmd.Parse(args[1:])
	}

	cfg, err := config.Load()
	if err != nil {
		outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		return
	}

	if *set != "" {
		setKeywords(cfg, *set, jsonFlag)
	} else if *list {
		listKeywords(cfg, jsonFlag)
	} else if *clear {
		clearKeywords(cfg, jsonFlag)
	} else {
		if jsonFlag {
			outputJSON(map[string]string{
				"error": "Use --set, --list, or --clear",
			})
		} else {
			fmt.Println("Keyword management commands:")
			fmt.Println("  tmux-stt keywords --set \"hey,computer,toto\"  # Set wake words")
			fmt.Println("  tmux-stt keywords --list                   # List current keywords")
			fmt.Println("  tmux-stt keywords --clear                 # Clear keywords")
		}
	}
}

func handleEngine(jsonFlag bool) {
	engineCmd := flag.NewFlagSet("engine", flag.ExitOnError)
	set := engineCmd.String("set", "", "Set engine (vad-method:energy|silero, wake-method:stt|kws)")
	list := engineCmd.Bool("list", false, "List current engine settings")
	
	args := flag.Args()
	if len(args) > 1 {
		engineCmd.Parse(args[1:])
	}

	cfg, err := config.Load()
	if err != nil {
		outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		return
	}

	if *set != "" {
		setEngine(cfg, *set, jsonFlag)
	} else if *list {
		listEngine(cfg, jsonFlag)
	} else {
		if jsonFlag {
			outputJSON(map[string]string{
				"error": "Use --set or --list",
			})
		} else {
			fmt.Println("Engine management commands:")
			fmt.Println("  tmux-stt engine --set vad-method=silero  # Set VAD method")
			fmt.Println("  tmux-stt engine --set wake-method=kws    # Set wake word method")
			fmt.Println("  tmux-stt engine --list                  # List current engine settings")
		}
	}
}

func downloadModel(model string, jsonFlag bool) {
	var scriptPath string
	var modelName string
	
	switch model {
	case "vad-silero", "silero", "vad":
		scriptPath = "./scripts/download-silero-vad.sh"
		modelName = "Silero VAD"
	case "kws", "keyword-spotting":
		scriptPath = "./scripts/download-kws-model.sh"
		modelName = "Keyword Spotting"
	default:
		outputError(jsonFlag, "Unknown model type", ExitInvalidArgument, fmt.Errorf("model: %s", model))
		return
	}

	if jsonFlag {
		outputJSON(map[string]interface{}{
			"action": "download",
			"model": model,
			"script": scriptPath,
			"status": "running",
		})
	} else {
		fmt.Printf("📦 Downloading %s model...\n", modelName)
		fmt.Printf("Script: %s\n", scriptPath)
	}

	// Execute the download script
	cmd := exec.Command("bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		outputError(jsonFlag, "Failed to download model", ExitInternalError, err)
		return
	}

	if jsonFlag {
		outputJSON(map[string]interface{}{
			"action": "download",
			"model": model,
			"status": "completed",
		})
	} else {
		fmt.Printf("✅ %s model downloaded successfully\n", modelName)
	}
}

func listModels(jsonFlag bool) {
	homeDir, _ := os.UserHomeDir()
	modelsDir := filepath.Join(homeDir, ".local", "share", "tmux-stt")
	
	models := []struct {
		name string
		path string
		installed bool
	}{
		{"Silero VAD", filepath.Join(modelsDir, "models", "silero_vad.onnx"), false},
		{"KWS Encoder", filepath.Join(modelsDir, "sherpa-kws", "encoder-epoch-13-avg-2-chunk-16-left-64.onnx"), false},
		{"KWS Decoder", filepath.Join(modelsDir, "sherpa-kws", "decoder-epoch-13-avg-2-chunk-16-left-64.onnx"), false},
		{"KWS Joiner", filepath.Join(modelsDir, "sherpa-kws", "joiner-epoch-13-avg-2-chunk-16-left-64.onnx"), false},
		{"KWS Tokens", filepath.Join(modelsDir, "sherpa-kws", "tokens.txt"), false},
	}

	for i := range models {
		if _, err := os.Stat(models[i].path); err == nil {
			models[i].installed = true
		}
	}

	if jsonFlag {
		outputJSON(models)
	} else {
		fmt.Println("Available models:")
		for _, model := range models {
			status := "❌ Not installed"
			if model.installed {
				status = "✅ Installed"
			}
			fmt.Printf("  %s: %s\n", model.name, status)
		}
	}
}

func modelStatus(jsonFlag bool) {
	cfg, err := config.Load()
	if err != nil {
		outputError(jsonFlag, "Failed to load config", ExitConfigError, err)
		return
	}

	status := map[string]interface{}{
		"vad_method": cfg.VAD.Method,
		"wake_word_method": cfg.WakeWordMethod,
	}

	// Check model availability
	homeDir, _ := os.UserHomeDir()
	sileroPath := filepath.Join(homeDir, ".local", "share", "tmux-stt", "models", "silero_vad.onnx")
	kwsEncoderPath := filepath.Join(homeDir, ".local", "share", "tmux-stt", "sherpa-kws", "encoder-epoch-13-avg-2-chunk-16-left-64.onnx")
	
	status["silero_vad_installed"] = fileExists(sileroPath)
	status["kws_models_installed"] = fileExists(kwsEncoderPath)

	if jsonFlag {
		outputJSON(status)
	} else {
		fmt.Println("Engine Status:")
		fmt.Printf("  VAD Method: %s\n", cfg.VAD.Method)
		fmt.Printf("  Wake Word Method: %s\n", cfg.WakeWordMethod)
		fmt.Printf("  Silero VAD: %v\n", status["silero_vad_installed"])
		fmt.Printf("  KWS Models: %v\n", status["kws_models_installed"])
	}
}

func setKeywords(cfg *config.Config, keywordsStr string, jsonFlag bool) {
	keywords := strings.Split(keywordsStr, ",")
	for i := range keywords {
		keywords[i] = strings.TrimSpace(keywords[i])
	}

	// Create keywords file
	homeDir, _ := os.UserHomeDir()
	keywordsFile := filepath.Join(homeDir, ".local", "share", "tmux-stt", "sherpa-kws", "keywords.txt")
	
	content := ""
	for i, keyword := range keywords {
		if i > 0 {
			content += "\n"
		}
		content += fmt.Sprintf("%s @%s", keyword, keyword)
	}

	// Ensure directory exists
	keywordsDir := filepath.Dir(keywordsFile)
	if err := os.MkdirAll(keywordsDir, 0755); err != nil {
		outputError(jsonFlag, "Failed to create keywords directory", ExitInternalError, err)
		return
	}

	if err := os.WriteFile(keywordsFile, []byte(content), 0644); err != nil {
		outputError(jsonFlag, "Failed to write keywords file", ExitInternalError, err)
		return
	}

	// Update config
	cfg.KWS.KeywordsFile = keywordsFile
	if err := config.Save(cfg); err != nil {
		outputError(jsonFlag, "Failed to save config", ExitConfigError, err)
		return
	}

	if jsonFlag {
		outputJSON(map[string]interface{}{
			"keywords": keywords,
			"file": keywordsFile,
			"status": "set",
		})
	} else {
		fmt.Printf("✅ Keywords set: %v\n", keywords)
		fmt.Printf("📝 Keywords file: %s\n", keywordsFile)
	}
}

func listKeywords(cfg *config.Config, jsonFlag bool) {
	keywordsFile := cfg.KWS.KeywordsFile
	if keywordsFile == "" {
		homeDir, _ := os.UserHomeDir()
		keywordsFile = filepath.Join(homeDir, ".local", "share", "tmux-stt", "sherpa-kws", "keywords.txt")
	}

	if _, err := os.Stat(keywordsFile); os.IsNotExist(err) {
		if jsonFlag {
			outputJSON(map[string]interface{}{
				"keywords": []string{},
				"file": keywordsFile,
				"status": "no_keywords",
			})
		} else {
			fmt.Println("No keywords file found")
			fmt.Printf("Expected location: %s\n", keywordsFile)
		}
		return
	}

	content, err := os.ReadFile(keywordsFile)
	if err != nil {
		outputError(jsonFlag, "Failed to read keywords file", ExitInternalError, err)
		return
	}

	// Parse keywords from file
	lines := strings.Split(string(content), "\n")
	var keywords []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Extract keyword from format "keyword @keyword"
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				keywords = append(keywords, parts[0])
			}
		}
	}

	if jsonFlag {
		outputJSON(map[string]interface{}{
			"keywords": keywords,
			"file": keywordsFile,
			"count": len(keywords),
		})
	} else {
		fmt.Printf("Current keywords (%d):\n", len(keywords))
		for _, keyword := range keywords {
			fmt.Printf("  - %s\n", keyword)
		}
	}
}

func clearKeywords(cfg *config.Config, jsonFlag bool) {
	keywordsFile := cfg.KWS.KeywordsFile
	if keywordsFile == "" {
		homeDir, _ := os.UserHomeDir()
		keywordsFile = filepath.Join(homeDir, ".local", "share", "tmux-stt", "sherpa-kws", "keywords.txt")
	}

	if err := os.Remove(keywordsFile); err != nil && !os.IsNotExist(err) {
		outputError(jsonFlag, "Failed to remove keywords file", ExitInternalError, err)
		return
	}

	if jsonFlag {
		outputJSON(map[string]interface{}{
			"status": "cleared",
			"file": keywordsFile,
		})
	} else {
		fmt.Println("✅ Keywords cleared")
	}
}

func setEngine(cfg *config.Config, setting string, jsonFlag bool) {
	parts := strings.SplitN(setting, "=", 2)
	if len(parts) != 2 {
		outputError(jsonFlag, "Invalid setting format", ExitInvalidArgument, fmt.Errorf("use: key=value"))
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	switch key {
	case "vad-method", "vad":
		if value != "energy" && value != "silero" {
			outputError(jsonFlag, "Invalid VAD method", ExitInvalidArgument, fmt.Errorf("must be 'energy' or 'silero'"))
			return
		}
		cfg.VAD.Method = value
	case "wake-method", "wake-word-method", "wake":
		if value != "stt" && value != "kws" {
			outputError(jsonFlag, "Invalid wake word method", ExitInvalidArgument, fmt.Errorf("must be 'stt' or 'kws'"))
			return
		}
		cfg.WakeWordMethod = value
	default:
		outputError(jsonFlag, "Unknown setting", ExitInvalidArgument, fmt.Errorf("key: %s", key))
		return
	}

	if err := config.Save(cfg); err != nil {
		outputError(jsonFlag, "Failed to save config", ExitConfigError, err)
		return
	}

	if jsonFlag {
		outputJSON(map[string]interface{}{
			"setting": key,
			"value": value,
			"status": "set",
		})
	} else {
		fmt.Printf("✅ Engine setting updated: %s = %s\n", key, value)
	}
}

func listEngine(cfg *config.Config, jsonFlag bool) {
	if jsonFlag {
		outputJSON(map[string]interface{}{
			"vad_method": cfg.VAD.Method,
			"wake_word_method": cfg.WakeWordMethod,
		})
	} else {
		fmt.Println("Current engine settings:")
		fmt.Printf("  VAD Method: %s\n", cfg.VAD.Method)
		fmt.Printf("  Wake Word Method: %s\n", cfg.WakeWordMethod)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

