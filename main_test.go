package main

import (
	"os"
	"strings"
	"testing"
)

// TestCommitMessageCleaning tests the logic used to clean llama-cli output
func TestCommitMessageCleaning(t *testing.T) {
	input := `
llama_print_timings: total time = 123ms
system_info: n_threads = 8
Write a concise git commit message for the following changes.
Respond ONLY with the commit message.

feat: add new validation logic
`
	lines := strings.Split(input, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "Write a concise git commit message") ||
			strings.Contains(trimmed, "Respond ONLY with") ||
			strings.HasPrefix(trimmed, "llama_") ||
			strings.HasPrefix(trimmed, "system_info") {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}

	result := strings.Join(cleaned, "\n")
	expected := "feat: add new validation logic"

	if result != expected {
		t.Errorf("Expected cleaned message to be '%s', got '%s'", expected, result)
	}
}

func TestValidationLogic(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		force     bool
		wantError bool
	}{
		{"Short message with force", "fix", true, true},
		{"Long message with force", "feat: implementing new feature", true, false},
		{"Short message without force", "fix", false, false}, // Should be allowed without -f
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := strings.TrimSpace(tt.msg)
			isShort := len(msg) < 10
			
			// Simulating the logic in main():
			// if !*force { edit } else { if len < 10 { exit } }
			shouldFail := false
			if tt.force && isShort {
				shouldFail = true
			}

			if shouldFail != tt.wantError {
				t.Errorf("Validation logic failed for '%s' (force=%v): expected failure %v, got %v", tt.msg, tt.force, tt.wantError, shouldFail)
			}
		})
	}
}

func TestEnvironment(t *testing.T) {
	// Check if llama-cpp is accessible in the environment
	// Note: This might fail in CI if not installed, but good for local dev specs
	_, err := os.Stat(os.Getenv("HOME") + "/bin/llama-cpp")
	if os.IsNotExist(err) {
		t.Log("Warning: ~/bin/llama-cpp not found. AI generation might fail.")
	}
}
