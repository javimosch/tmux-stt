package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	pidFile = "/tmp/tmux-stt.pid"
	logFile = "/tmp/tmux-stt.log"
)

func startDaemonProcess(wakeWord, modelSize string, jsonFlag bool) {
	// Check if already running
	if isDaemonRunning() {
		if jsonFlag {
			output := map[string]interface{}{
				"status":  "already_running",
				"message": "Daemon is already running",
			}
			outputJSON(output)
		} else {
			fmt.Println("Daemon is already running")
		}
		return
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    115,
					"type":    "internal_error",
					"message": "Failed to get executable path",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": false,
				},
			}
			outputJSON(output)
		} else {
			fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		}
		os.Exit(1)
	}

	// Create command to run server in foreground
	cmd := exec.Command(execPath, "start", "-wake-word", wakeWord, "-model", modelSize)

	// Set up logging
	logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    115,
					"type":    "internal_error",
					"message": "Failed to open log file",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": false,
				},
			}
			outputJSON(output)
		} else {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		}
		os.Exit(1)
	}
	defer logFileHandle.Close()

	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	// Start the process
	if err := cmd.Start(); err != nil {
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    115,
					"type":    "internal_error",
					"message": "Failed to start daemon",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": false,
				},
			}
			outputJSON(output)
		} else {
			fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
		}
		os.Exit(1)
	}

	// Write PID file
	pid := cmd.Process.Pid
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    115,
					"type":    "internal_error",
					"message": "Failed to write PID file",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": false,
				},
			}
			outputJSON(output)
		} else {
			fmt.Fprintf(os.Stderr, "Error writing PID file: %v\n", err)
		}
		cmd.Process.Kill()
		os.Exit(1)
	}

	if jsonFlag {
		output := map[string]interface{}{
			"status":  "started",
			"pid":     pid,
			"logFile": logFile,
			"message": "Daemon started successfully",
		}
		outputJSON(output)
	} else {
		fmt.Printf("Daemon started with PID %d\n", pid)
		fmt.Printf("Logs: %s\n", logFile)
		fmt.Println("Note: Run this command inside a tmux session for full functionality")
	}
}

func stopDaemon(jsonFlag bool) {
	// Read PID file
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			if jsonFlag {
				output := map[string]interface{}{
					"status":  "not_running",
					"message": "Daemon is not running",
				}
				outputJSON(output)
			} else {
				fmt.Println("❌ Daemon is not running")
			}
			return
		}
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    92,
					"type":    "resource_error",
					"message": "Failed to read PID file",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": false,
				},
			}
			outputJSON(output)
		} else {
			fmt.Fprintf(os.Stderr, "Error reading PID file: %v\n", err)
		}
		os.Exit(1)
	}

	var pid int
	fmt.Sscanf(string(pidData), "%d", &pid)

	// Send SIGTERM to the process
	process, err := os.FindProcess(pid)
	if err != nil {
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    115,
					"type":    "internal_error",
					"message": "Failed to find process",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": false,
				},
			}
			outputJSON(output)
		} else {
			fmt.Fprintf(os.Stderr, "Error finding process: %v\n", err)
		}
		os.Exit(1)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		if jsonFlag {
			output := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    115,
					"type":    "internal_error",
					"message": "Failed to stop process",
					"details": map[string]interface{}{
						"original_error": err.Error(),
					},
					"recoverable": false,
				},
			}
			outputJSON(output)
		} else {
			fmt.Fprintf(os.Stderr, "Error stopping process: %v\n", err)
		}
		os.Exit(1)
	}

	// Remove PID file
	os.Remove(pidFile)

	if jsonFlag {
		output := map[string]interface{}{
			"status":  "stopped",
			"pid":     pid,
			"message": "Daemon stopped successfully",
		}
		outputJSON(output)
	} else {
		fmt.Printf("Daemon stopped (PID %d)\n", pid)
	}
}

func checkDaemonStatus(jsonFlag bool) {
	if isDaemonRunning() {
		pidData, _ := os.ReadFile(pidFile)
		if jsonFlag {
			output := map[string]interface{}{
				"status":  "running",
				"pid":     string(pidData),
				"logFile": logFile,
			}
			outputJSON(output)
		} else {
			fmt.Printf("Daemon is running (PID %s)\n", string(pidData))
			fmt.Printf("Logs: %s\n", logFile)
		}
	} else {
		if jsonFlag {
			output := map[string]interface{}{
				"status":  "not_running",
				"message": "Daemon is not running",
			}
			outputJSON(output)
		} else {
			fmt.Println("❌ Daemon is not running")
		}
	}
}

func isDaemonRunning() bool {
	// Check if PID file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false
	}

	// Read PID file
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	var pid int
	fmt.Sscanf(string(pidData), "%d", &pid)

	// Check if process is running
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process not running, clean up PID file
		os.Remove(pidFile)
		return false
	}

	return true
}

func getExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Resolve symlinks
	resolvedPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return execPath, nil
	}

	return resolvedPath, nil
}