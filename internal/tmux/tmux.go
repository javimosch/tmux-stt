package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type TmuxClient struct {
	socketPath   string
	autoEnter    bool
	targetPane   string
	targetWindow string
}

func NewTmuxClient(socketPath string, autoEnter bool) *TmuxClient {
	return &TmuxClient{
		socketPath:   socketPath,
		autoEnter:    autoEnter,
		targetPane:   "",
		targetWindow: "",
	}
}

func (t *TmuxClient) SetTargetPane(targetPane string) {
	t.targetPane = targetPane
}

func (t *TmuxClient) SetTargetWindow(targetWindow string) {
	t.targetWindow = targetWindow
}

func (t *TmuxClient) buildCommand(args ...string) *exec.Cmd {
	if t.socketPath != "" {
		args = append([]string{"-L", t.socketPath}, args...)
	}
	return exec.Command("tmux", args...)
}

func (t *TmuxClient) buildTarget() string {
	// Prefer pane specification over window
	if t.targetPane != "" {
		return t.targetPane
	}
	if t.targetWindow != "" {
		return t.targetWindow
	}
	return "" // Use active pane/window
}

func (t *TmuxClient) SendKeys(text string) error {
	// Escape special tmux characters
	escaped := strings.ReplaceAll(text, ";", "\\;")
	
	// Build target specification
	target := t.buildTarget()
	
	// Send keys to target pane
	args := []string{"send-keys"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, escaped, "Enter")
	
	cmd := t.buildCommand(args...)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send keys to tmux: %w", err)
	}

	return nil
}

func (t *TmuxClient) SendKeysNoEnter(text string) error {
	// Escape special tmux characters
	escaped := strings.ReplaceAll(text, ";", "\\;")
	
	// Build target specification
	target := t.buildTarget()
	
	// Send keys to target pane without Enter
	args := []string{"send-keys"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, escaped)
	
	cmd := t.buildCommand(args...)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send keys to tmux: %w", err)
	}

	return nil
}

func (t *TmuxClient) PasteText(text string) error {
	// Use tmux paste-buffer for better handling of multi-line text
	cmd := t.buildCommand("set-buffer", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set tmux buffer: %w", err)
	}

	// Paste the buffer
	cmd = t.buildCommand("paste-buffer", "-d")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to paste tmux buffer: %w", err)
	}

	return nil
}

func (t *TmuxClient) IsInsideTmux() bool {
	// Check if TMUX environment variable is set
	return os.Getenv("TMUX") != ""
}

func (t *TmuxClient) GetCurrentSession() (string, error) {
	cmd := t.buildCommand("display-message", "-p", "#S")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current session: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (t *TmuxClient) GetCurrentWindow() (string, error) {
	cmd := t.buildCommand("display-message", "-p", "#W")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current window: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (t *TmuxClient) GetCurrentPane() (string, error) {
	cmd := t.buildCommand("display-message", "-p", "#P")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current pane: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// ListSessions lists all tmux sessions
func (t *TmuxClient) ListSessions() ([]string, error) {
	cmd := t.buildCommand("list-sessions")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var sessions []string
	for _, line := range lines {
		if line != "" {
			// Extract session name (format: "session-name: [windows]")
			parts := strings.Split(line, ":")
			if len(parts) > 0 {
				sessions = append(sessions, strings.TrimSpace(parts[0]))
			}
		}
	}

	return sessions, nil
}

// ListWindows lists all windows in the current session
func (t *TmuxClient) ListWindows() ([]string, error) {
	cmd := t.buildCommand("list-windows")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list windows: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var windows []string
	for _, line := range lines {
		if line != "" {
			// Extract window index and name
			parts := strings.Split(line, " ")
			if len(parts) > 1 {
				windows = append(windows, strings.TrimSpace(parts[1]))
			}
		}
	}

	return windows, nil
}

// ListPanes lists all panes in the current window
func (t *TmuxClient) ListPanes() ([]string, error) {
	cmd := t.buildCommand("list-panes")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list panes: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var panes []string
	for _, line := range lines {
		if line != "" {
			panes = append(panes, strings.TrimSpace(line))
		}
	}

	return panes, nil
}
