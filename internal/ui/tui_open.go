package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/yesahem/burnenv/internal/crypto"
)

type openResult struct {
	plaintext []byte
	err       error
}

type openModel struct {
	passwordInput textinput.Model
	payload       *crypto.EncryptedPayload
	result        *openResult
	quitting      bool
	width         int
	height        int
}

func newOpenModel(payload *crypto.EncryptedPayload) openModel {
	pi := textinput.New()
	pi.Placeholder = "••••••••"
	pi.Width = 72
	pi.EchoMode = textinput.EchoPassword

	return openModel{
		passwordInput: pi,
		payload:       payload,
		width:         80,
		height:        24,
	}
}

func (m openModel) inputWidth() int {
	w := m.width - 8
	if w < 40 {
		w = 40
	}
	return w
}

func (m openModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m openModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.passwordInput.Width = m.inputWidth()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if m.result != nil {
				return m, tea.Quit
			}
			return m, m.doDecrypt
		}
	case openResult:
		m.result = &msg
		return m, nil
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m openModel) doDecrypt() tea.Msg {
	password := m.passwordInput.Value()
	if password == "" {
		return openResult{err: fmt.Errorf("password cannot be empty")}
	}
	plaintext, err := crypto.Decrypt(m.payload, password)
	if err != nil {
		return openResult{err: err}
	}
	return openResult{plaintext: plaintext}
}

func (m openModel) View() string {
	if m.quitting {
		return ""
	}
	boxWidth := m.width - 4
	if boxWidth < 50 {
		boxWidth = 50
	}

	var content string
	if m.result != nil {
		if m.result.err != nil {
			content = Error.Render("Error: "+m.result.err.Error()) + "\n"
		} else {
			var b strings.Builder
			b.WriteString(Success.Render("✓ Secret unlocked.\n\n"))
			b.WriteString(Box.Width(boxWidth - 4).Render(string(m.result.plaintext)))
			b.WriteString("\n\n")
			b.WriteString(Burn.Render("🔥 Secret retrieved and burned. One-time use complete."))
			content = b.String()
		}
	} else {
		var b strings.Builder
		b.WriteString(Title.Render("BurnEnv Open") + "\n\n")
		b.WriteString(Prompt.Render("Enter password to unlock:") + "\n")
		b.WriteString(m.passwordInput.View() + "\n\n")
		b.WriteString(Muted.Render("Enter to decrypt • Ctrl+C to cancel"))
		content = Box.Width(boxWidth).Render(b.String())
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// RunOpenTUI launches the Bubble Tea TUI for password input and displays result.
func RunOpenTUI(payload *crypto.EncryptedPayload) ([]byte, error) {
	m := newOpenModel(payload)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := final.(openModel)
	if fm.result != nil && fm.result.err != nil {
		return nil, fm.result.err
	}
	if fm.result != nil {
		return fm.result.plaintext, nil
	}
	return nil, fmt.Errorf("cancelled")
}

// PrintPlaintextToStdout ensures plaintext goes to stdout for piping.
func PrintPlaintextToStdout(plaintext []byte) {
	os.Stdout.Write(plaintext)
	if len(plaintext) > 0 && plaintext[len(plaintext)-1] != '\n' {
		os.Stdout.Write([]byte{'\n'})
	}
}
