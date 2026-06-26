# iacp - Intelligent Add Commit Push

`iacp` is a Go-based CLI tool that automates the standard Git workflow (`add`, `commit`, `pull`, `push`) while using local or cloud-based AI models to generate meaningful commit messages based on your current changes.

## Features

- **Multi-Provider AI Support**:
  - **llama.cpp**: Run local GGUF models via `llama-cli`.
  - **Ollama**: Seamlessly use models from your local Ollama instance.
  - **AI-gatiator**: Access free cloud models (Gemini, Llama, Minimax, etc.) via a local gateway.
- **TUI Model Selector**: Run `iacp -s` to select your preferred AI provider and model using a beautiful terminal interface (Bubble Tea).
- **Context Awareness**: Displays model context limits and warns you if your `git diff` might exceed them, suggesting alternative models.
- **Diff Preview**: Shows your staged changes before processing, so you know exactly what the AI is describing.
- **Interactive Review**: Automatically opens your favorite editor (from `$EDITOR` or `vi`) to let you refine the generated message.
- **Fast Mode**: Use the `-f` flag to bypass the editor and commit/push instantly (enforces a 10-character minimum).
- **Safety First**: Supports `Ctrl+C` to gracefully terminate AI generation and sub-processes.

## Installation

### Prerequisites
- [Go](https://golang.org/doc/install) (to build from source)
- `llama.cpp` (for local GGUF support)
- `Ollama` (optional, for Ollama support)
- `AI-gatiator` (optional, for free cloud models)

### Build and Install
```bash
git clone <repository-url>
cd iacp
go build -o iacp main.go
mkdir -p ~/bin
cp iacp ~/bin/iacp
chmod +x ~/bin/iacp
```
Make sure `~/bin` is in your `$PATH`.

## Usage

### 1. Select a Model
The first time you use `iacp`, or when you want to switch providers, run:
```bash
iacp -s
```
This will scan for local GGUF models, Ollama models, and query the Gatiator API (at `localhost:1313`) to build a list for you.

### 2. Normal Workflow (Edit & Commit)
```bash
iacp
```
- Stages all changes (`git add -A .`).
- Shows the diff.
- Generates a suggested commit message using your selected AI.
- Opens your editor for review.
- After saving and closing the editor, it runs `git commit`, `git pull`, and `git push`.

### 3. Force Mode (Instant Commit)
```bash
iacp -f
```
- Skips the editor step.
- Automatically commits if the generated message is at least 10 characters long.
- Useful for quick, straightforward updates.

## Configuration
Settings are stored in JSON format at:
`~/.config/iacp/config.json`

## Development & Specs
The project includes a test suite to verify core logic like message cleaning and validation:
```bash
cd iacp
go test -v
```

## License
MIT
