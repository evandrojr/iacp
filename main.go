package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Config struct {
	Provider     string `json:"provider"`
	ModelPath    string `json:"model_path"`
	ContextLimit int    `json:"context_limit"`
}

func main() {
	force := flag.Bool("f", false, "Force commit without editing")
	selectModel := flag.Bool("s", false, "Select AI model")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\nAborting...")
		cancel()
		os.Exit(1)
	}()

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "iacp")
	configPath := filepath.Join(configDir, "config.json")

	if *selectModel {
		runModelSelector(configDir, configPath)
		return
	}

	runCommand(ctx, "git", "add", "-A", ".")
	diff, err := getCommandOutput(ctx, "git", "diff", "--cached")
	if err != nil {
		fmt.Printf("Error getting git diff: %v\n", err)
		os.Exit(1)
	}

	if len(strings.TrimSpace(diff)) == 0 {
		fmt.Println("No changes to commit.")
		os.Exit(0)
	}

	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("--- Current Changes ---"))
	fmt.Println(diff)
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("-----------------------"))

	config := loadConfig(configPath)
	
	// Show currently used model
	modelName := config.ModelPath
	if modelName == "" {
		if config.Provider == "gatiator" {
			modelName = "Auto-routed"
		} else {
			modelName = "Default"
		}
	}
	fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("Using AI: %s (%s)\n"), modelName, config.Provider)
	
	// Check context
	diffSize := len(diff)
	// Estimation: 1 token approx 4 chars. So context limit in chars is contextLimit * 4.
	// We'll be conservative and use contextLimit * 3.
	charLimit := config.ContextLimit * 3
	if config.ContextLimit > 0 && diffSize > charLimit {
		fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("\nWarning: Diff size (%d chars) might exceed model context limit (%d tokens).\n"), diffSize, config.ContextLimit)
		fmt.Println("Suggested: Run 'iacp -s' to select a model with a larger context.")
		fmt.Print("Continue anyway? (y/N): ")
		var resp string
		fmt.Scanln(&resp)
		if strings.ToLower(resp) != "y" {
			os.Exit(0)
		}
	}

	fmt.Println("Generating commit message...")
	commitMsg := generateCommitMessage(ctx, diff, config)
	if commitMsg == "" || strings.Contains(commitMsg, "Gatiator") || strings.Contains(commitMsg, "indisponíveis") {
		if commitMsg != "" {
			fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("AI Error: %s\n"), commitMsg)
		} else {
			fmt.Println("Failed to generate commit message.")
		}
		
		if config.Provider == "gatiator" {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("\nTip: Gatiator (Cloud) seems to be offline or unavailable."))
			fmt.Println("Suggested: Run 'iacp -s' and select a LOCAL model (llama.cpp or ollama) to continue.")
		}
		os.Exit(1)
	}

	if !*force {
		commitMsg, _ = editMessage(ctx, commitMsg)

		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("\n--- Review Message ---"))
		fmt.Println(commitMsg)
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("----------------------"))
		fmt.Print("Proceed with commit and push? (Y/n): ")
		
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "" && strings.ToLower(confirm) != "y" {
			fmt.Println("Aborted by user.")
			os.Exit(0)
		}
	} else {
		commitMsg = strings.TrimSpace(commitMsg)
		if len(commitMsg) < 10 {
			fmt.Printf("Commit message too short (%d characters). Minimum 10 characters required for auto-commit.\n", len(commitMsg))
			os.Exit(1)
		}
	}

	commitMsg = strings.TrimSpace(commitMsg)
	runCommand(ctx, "git", "commit", "-m", commitMsg)
	fmt.Println("Pulling...")
	runCommand(ctx, "git", "pull")
	fmt.Println("Pushing...")
	runCommand(ctx, "git", "push")
	fmt.Println("Done!")
}

func loadConfig(path string) Config {
	var config Config
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &config)
	}
	return config
}

func saveConfig(path string, config Config) {
	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(path, data, 0644)
}

func runCommand(ctx context.Context, name string, args ...string) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil && ctx.Err() != context.Canceled {
		fmt.Printf("Error running %s %v: %v\n", name, args, err)
		os.Exit(1)
	}
}

func getCommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

func generateCommitMessage(ctx context.Context, diff string, config Config) string {
	prompt := "Write a concise git commit message for the following changes. Respond ONLY with the commit message, no explanation or conversational text.\n\n" + diff
	switch config.Provider {
	case "ollama":
		return callOllama(ctx, prompt, config.ModelPath)
	case "gatiator":
		return callGatiator(ctx, prompt, config.ModelPath)
	default:
		return callLlama(ctx, prompt, config.ModelPath)
	}
}

func callLlama(ctx context.Context, prompt, modelPath string) string {
	args := []string{"--", "-p", prompt, "--no-display-prompt", "--n-predict", "100"}
	if modelPath != "" {
		args = append(args, "-m", modelPath)
	}
	cmd := exec.CommandContext(ctx, "llama-cpp", args...)
	var out bytes.Buffer
	if err := cmd.Run(); err != nil {
		return ""
	}
	return cleanAIOutput(out.String())
}

func callOllama(ctx context.Context, prompt, modelName string) string {
	if modelName == "" {
		modelName = "llama3"
	}
	cmd := exec.CommandContext(ctx, "ollama", "run", modelName, prompt)
	var out bytes.Buffer
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func callGatiator(ctx context.Context, prompt, modelName string) string {
	payload := map[string]interface{}{"messages": []map[string]string{{"role": "user", "content": prompt}}}
	if modelName != "" {
		payload["model"] = modelName
	}
	jsonData, _ := json.Marshal(payload)
	cmd := exec.CommandContext(ctx, "curl", "-s", "-X", "POST", "http://localhost:1313/v1/chat/completions", "-H", "Content-Type: application/json", "-d", string(jsonData))
	req, err := cmd.Output()
	if err != nil {
		return "Gatiator connection error"
	}

	// Check for API error response first
	var errRes struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(req, &errRes); err == nil && errRes.Error != "" {
		return errRes.Error
	}

	var res struct{ Choices []struct{ Message struct{ Content string } } }
	if err := json.Unmarshal(req, &res); err != nil || len(res.Choices) == 0 {
		return "Gatiator API response error"
	}
	return strings.TrimSpace(res.Choices[0].Message.Content)
}
func cleanAIOutput(result string) string {
	lines := strings.Split(result, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "llama_") || strings.HasPrefix(trimmed, "system_info") || strings.Contains(trimmed, "Write a concise git commit message") {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) == 0 {
		return ""
	}
	return cleaned[len(cleaned)-1]
}

type commitEditor struct {
	text      []string
	cursorRow int
	cursorCol int
	done      bool
	confirmed bool
}

func (m commitEditor) Init() tea.Cmd { return nil }

func (m commitEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			m.confirmed = false
			return m, tea.Quit
		case "ctrl+s", "f2":
			m.done = true
			m.confirmed = true
			return m, tea.Quit
		case "enter":
			line := m.text[m.cursorRow]
			before := line[:m.cursorCol]
			after := line[m.cursorCol:]
			m.text[m.cursorRow] = before
			newLines := make([]string, len(m.text)+1)
			copy(newLines, m.text[:m.cursorRow+1])
			newLines[m.cursorRow+1] = after
			copy(newLines[m.cursorRow+2:], m.text[m.cursorRow+1:])
			m.text = newLines
			m.cursorRow++
			m.cursorCol = 0
		case "backspace":
			if m.cursorCol > 0 {
				line := m.text[m.cursorRow]
				m.text[m.cursorRow] = line[:m.cursorCol-1] + line[m.cursorCol:]
				m.cursorCol--
			} else if m.cursorRow > 0 {
				prevLen := len(m.text[m.cursorRow-1])
				m.text[m.cursorRow-1] += m.text[m.cursorRow]
				m.text = append(m.text[:m.cursorRow], m.text[m.cursorRow+1:]...)
				m.cursorRow--
				m.cursorCol = prevLen
			}
		case "delete":
			line := m.text[m.cursorRow]
			if m.cursorCol < len(line) {
				m.text[m.cursorRow] = line[:m.cursorCol] + line[m.cursorCol+1:]
			} else if m.cursorRow < len(m.text)-1 {
				m.text[m.cursorRow] += m.text[m.cursorRow+1]
				m.text = append(m.text[:m.cursorRow+1], m.text[m.cursorRow+2:]...)
			}
		case "up":
			if m.cursorRow > 0 {
				m.cursorRow--
				if m.cursorCol > len(m.text[m.cursorRow]) {
					m.cursorCol = len(m.text[m.cursorRow])
				}
			}
		case "down":
			if m.cursorRow < len(m.text)-1 {
				m.cursorRow++
				if m.cursorCol > len(m.text[m.cursorRow]) {
					m.cursorCol = len(m.text[m.cursorRow])
				}
			}
		case "left":
			if m.cursorCol > 0 {
				m.cursorCol--
			} else if m.cursorRow > 0 {
				m.cursorRow--
				m.cursorCol = len(m.text[m.cursorRow])
			}
		case "right":
			if m.cursorCol < len(m.text[m.cursorRow]) {
				m.cursorCol++
			} else if m.cursorRow < len(m.text)-1 {
				m.cursorRow++
				m.cursorCol = 0
			}
		case "home":
			m.cursorCol = 0
		case "end":
			m.cursorCol = len(m.text[m.cursorRow])
		default:
			if len(msg.String()) == 1 {
				line := m.text[m.cursorRow]
				m.text[m.cursorRow] = line[:m.cursorCol] + msg.String() + line[m.cursorCol:]
				m.cursorCol++
			}
		}
	}
	return m, nil
}

func (m commitEditor) View() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("--- Edit Commit Message (Ctrl+S: confirm, Esc: cancel) ---\n"))
	for i, line := range m.text {
		if i == m.cursorRow {
			before := line[:m.cursorCol]
			if m.cursorCol < len(line) {
				cursor := string(line[m.cursorCol])
				after := line[m.cursorCol+1:]
				b.WriteString(before)
				b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("4")).Render(cursor))
				b.WriteString(after)
			} else {
				b.WriteString(before)
				b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("4")).Render(" "))
			}
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Ctrl+S: confirm  |  Esc: cancel  |  arrows: navigate"))
	return b.String()
}

func editMessage(ctx context.Context, initialMsg string) (string, string) {
	lines := strings.Split(initialMsg, "\n")
	p := tea.NewProgram(commitEditor{text: lines})
	m, err := p.Run()
	if err != nil {
		return initialMsg, ""
	}
	editor := m.(commitEditor)
	if !editor.confirmed {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("Edit cancelled, using original message."))
		return initialMsg, ""
	}
	return strings.Join(editor.text, "\n"), ""
}

type item struct {
	provider     string
	name         string
	path         string
	contextLimit int
}

type modelSelector struct {
	items    []item
	cursor   int
	selected item
}

func (m modelSelector) Init() tea.Cmd { return nil }
func (m modelSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 { m.cursor-- }
		case "down", "j":
			if m.cursor < len(m.items)-1 { m.cursor++ }
		case "enter", " ":
			m.selected = m.items[m.cursor]
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m modelSelector) View() string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("Select a model for iacp:\n\n")
	lastProvider := ""
	for i, it := range m.items {
		if it.provider != lastProvider {
			s += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render("\n" + strings.ToUpper(it.provider) + ":\n")
			lastProvider = it.provider
		}
		cursor := " "
		if m.cursor == i {
			cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(">")
			s += fmt.Sprintf("%s %s (Ctx: %d)\n", cursor, lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(it.name), it.contextLimit)
		} else {
			s += fmt.Sprintf("%s %s (Ctx: %d)\n", cursor, it.name, it.contextLimit)
		}
	}
	s += "\n(q to quit, enter to select)\n"
	return s
}

func runModelSelector(configDir, configPath string) {
	var items []item
	// llama.cpp
	out, _ := exec.Command("/home/s932743005/lab/llama.cpp/build/bin/llama-cli", "--cache-list").Output()
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, ".") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Llama 3.1 usually 128k, but let's assume 32k for llama-cli default safety
				items = append(items, item{provider: "llama.cpp", name: parts[1], path: "", contextLimit: 32768})
			}
		}
	}
	// Ollama
	out, _ = exec.Command("ollama", "list").Output()
	lines = strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 || line == "" { continue }
		parts := strings.Fields(line)
		if len(parts) > 0 {
			ctxLimit := 4096 // default
			info, err := exec.Command("ollama", "show", parts[0], "--modelfile").Output()
			if err == nil && strings.Contains(string(info), "context length") {
				// parse if possible, or just use show output
			}
			// Hardcoded common ones for better UX
			if strings.Contains(parts[0], "llama3") { ctxLimit = 8192 }
			if strings.Contains(parts[0], "llama3.1") { ctxLimit = 128000 }
			items = append(items, item{provider: "ollama", name: parts[0], path: parts[0], contextLimit: ctxLimit})
		}
	}
	// Gatiator
	items = append(items, item{provider: "gatiator", name: "AI-gatiator (Auto)", path: "", contextLimit: 128000})
	resp, err := exec.Command("curl", "-s", "http://localhost:1313/v1/models").Output()
	if err == nil {
		var modelList struct{ Data []struct{ ID string } }
		if err := json.Unmarshal(resp, &modelList); err == nil {
			for _, m := range modelList.Data {
				ctx := 128000
				if strings.Contains(m.ID, "flash") { ctx = 1000000 }
				items = append(items, item{provider: "gatiator", name: m.ID, path: m.ID, contextLimit: ctx})
			}
		}
	}
	p := tea.NewProgram(modelSelector{items: items})
	m, _ := p.Run()
	sel := m.(modelSelector).selected
	if sel.name != "" {
		saveConfig(configPath, Config{Provider: sel.provider, ModelPath: sel.path, ContextLimit: sel.contextLimit})
		fmt.Printf("Selected: %s (Ctx: %d)\n", sel.name, sel.contextLimit)
	}
}
