package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yesahem/burnenv/internal/client"
	"github.com/yesahem/burnenv/internal/crypto"
	"github.com/yesahem/burnenv/internal/store"
)

type retrieveStep int

const (
	stepKey retrieveStep = iota
	stepPassword
	stepResult   // show content + copy/export
	stepExport   // waiting for path input
	stepBurned
)

type retrieveModel struct {
	width         int
	height        int
	step          retrieveStep
	keyInput      textinput.Model
	passwordInput textinput.Model
	pathInput     textinput.Model
	payload       *crypto.EncryptedPayload
	plaintext []byte
	err       error
	done      bool
}

func newRetrieveModel(width, height int) retrieveModel {
	ki := textinput.New()
	ki.Placeholder = "Paste secure key here..."
	ki.Width = 60
	ki.CharLimit = 500 // Allow long keys
	ki.Focus()         // Focus immediately

	pi := textinput.New()
	pi.Placeholder = "••••••••"
	pi.EchoMode = textinput.EchoPassword
	pi.Width = 60

	pathInput := textinput.New()
	pathInput.Placeholder = "e.g. ./myenv or /path/to/.env"
	pathInput.Width = 60

	return retrieveModel{
		width:         width,
		height:        height,
		step:          stepKey,
		keyInput:      ki,
		passwordInput: pi,
		pathInput:     pathInput,
	}
}

func (m retrieveModel) Init() tea.Cmd {
	return textinput.Blink
}

type fetchDone struct {
	payload *crypto.EncryptedPayload
	err     error
}

func (m retrieveModel) doFetch() tea.Msg {
	key := strings.TrimSpace(m.keyInput.Value())
	if key == "" {
		return fetchDone{err: fmt.Errorf("secure key cannot be empty")}
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		p, err := client.Get(key)
		return fetchDone{payload: p, err: err}
	}
	mock, err := store.NewMockStore("")
	if err != nil {
		return fetchDone{err: err}
	}
	p, err := mock.Load(key)
	return fetchDone{payload: p, err: err}
}

type decryptDone struct {
	plaintext []byte
	err       error
}

func (m retrieveModel) doDecrypt() tea.Msg {
	password := m.passwordInput.Value()
	if password == "" {
		return decryptDone{err: fmt.Errorf("password cannot be empty")}
	}
	plaintext, err := crypto.Decrypt(m.payload, password)
	return decryptDone{plaintext: plaintext, err: err}
}

type copyDone struct{}
type exportDone struct{ err error }

func (m retrieveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := m.width - 12
		if w < 40 {
			w = 40
		}
		m.keyInput.Width = w
		m.passwordInput.Width = w
		m.pathInput.Width = w
		return &m, nil

	case tea.KeyMsg:
		keyStr := msg.String()

		// Handle global keys first
		switch keyStr {
		case "ctrl+c":
			return &m, tea.Quit
		case "esc":
			if m.step == stepExport {
				m.pathInput.Blur()
				m.step = stepResult
				return &m, nil
			}
			m.done = true
			return &m, nil
		}

		// Comprehensive Enter/Return detection for Mac compatibility
		enterBinding := key.NewBinding(key.WithKeys("enter", "return", "ctrl+j", "ctrl+m"))
		isEnter := key.Matches(msg, enterBinding) ||
			msg.Type == tea.KeyEnter ||
			msg.Type == tea.KeyCtrlJ ||
			msg.Type == tea.KeyCtrlM ||
			keyStr == "enter" || keyStr == "return" ||
			keyStr == "\r" || keyStr == "\n"
		if !isEnter && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			r := msg.Runes[0]
			isEnter = r == '\r' || r == '\n'
		}

		// Space detection
		isSpace := msg.Type == tea.KeySpace || keyStr == " " || keyStr == "space"

		// Handle stepKey - Enter or Space to fetch
		if m.step == stepKey {
			if isEnter || isSpace {
				return &m, m.doFetch
			}
		}

		// Handle stepPassword - Enter or Space to decrypt
		if m.step == stepPassword {
			if isEnter || isSpace {
				return &m, m.doDecrypt
			}
		}

		// Handle stepResult - c to copy, e to export
		if m.step == stepResult {
			switch keyStr {
			case "c", "C":
				return &m, func() tea.Msg {
					clipboard.WriteAll(string(m.plaintext))
					return copyDone{}
				}
			case "e", "E":
				m.pathInput.Focus()
				m.pathInput.SetValue("")
				m.step = stepExport
				return &m, textinput.Blink
			}
		}

		// Handle stepExport - Enter to save
		if m.step == stepExport {
			if isEnter || isSpace {
				path := strings.TrimSpace(m.pathInput.Value())
				if path != "" {
					return &m, func() tea.Msg {
						err := os.WriteFile(path, m.plaintext, 0600)
						return exportDone{err: err}
					}
				}
			}
		}

		// Handle stepBurned - Enter or Space to go back
		if m.step == stepBurned {
			if isEnter || isSpace {
				m.done = true
				return &m, nil
			}
		}

	case fetchDone:
		if msg.err != nil {
			m.err = msg.err
			return &m, nil
		}
		m.payload = msg.payload
		m.keyInput.Blur()
		m.passwordInput.Focus()
		m.step = stepPassword
		return &m, textinput.Blink

	case decryptDone:
		if msg.err != nil {
			m.err = msg.err
			return &m, nil
		}
		m.plaintext = msg.plaintext
		m.passwordInput.Blur()
		m.step = stepResult
		return &m, nil

	case copyDone:
		m.step = stepBurned
		return &m, nil

	case exportDone:
		if msg.err != nil {
			m.err = msg.err
			m.step = stepResult
			return &m, nil
		}
		m.step = stepBurned
		return &m, nil
	}

	// Forward to focused input (for typing characters)
	if m.step == stepKey {
		var cmd tea.Cmd
		m.keyInput, cmd = m.keyInput.Update(msg)
		return &m, cmd
	}
	if m.step == stepPassword {
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return &m, cmd
	}
	if m.step == stepExport {
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		return &m, cmd
	}

	return &m, nil
}

func (m retrieveModel) View() string {
	// Use most of the terminal width
	boxWidth := m.width - 8
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 120 {
		boxWidth = 120 // Cap max width for readability
	}

	// Input width
	inputWidth := boxWidth - 8
	if inputWidth < 50 {
		inputWidth = 50
	}

	var b strings.Builder

	// Check if we have an error to display (burned/expired secrets)
	if m.err != nil {
		errMsg := m.err.Error()
		// Check if it's a "burned" or security-related message
		if strings.Contains(errMsg, "🔥") || strings.Contains(errMsg, "burned") ||
			strings.Contains(errMsg, "expired") || strings.Contains(errMsg, "not found") {
			b.WriteString(Title.Render("Retrieve env") + "\n\n")
			b.WriteString(Burn.Render(errMsg) + "\n\n")

			b.WriteString(Muted.Render("Esc to go back"))
		} else {
			b.WriteString(Title.Render("Retrieve env") + "\n\n")
			b.WriteString(Error.Render("Error: "+errMsg) + "\n\n")
			b.WriteString(Muted.Render("Esc to go back"))
		}
		content := Box.Width(boxWidth).Render(b.String())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	switch m.step {
	case stepKey:
		b.WriteString(Title.Render("Retrieve env") + "\n\n")
		b.WriteString(Prompt.Render("Paste secure key below:") + "\n")
		b.WriteString(Muted.Render("(Key will be visible for verification)") + "\n\n")
		m.keyInput.Width = inputWidth
		b.WriteString(m.keyInput.View())
		keyVal := strings.TrimSpace(m.keyInput.Value())
		keyLen := len(keyVal)
		if keyLen > 0 {
			b.WriteString("\n")
			b.WriteString(Success.Render(fmt.Sprintf("✓ %d characters", keyLen)))
			// Show a preview of the key (truncated if too long)
			if keyLen > 50 {
				b.WriteString("\n")
				b.WriteString(Muted.Render("Key: " + keyVal[:25] + "..." + keyVal[keyLen-20:]))
			}
		}
		b.WriteString("\n\n")
		b.WriteString(Muted.Render("Enter or Space to continue • Esc to cancel"))

	case stepPassword:
		b.WriteString(Title.Render("Retrieve env") + "\n\n")
		b.WriteString(Prompt.Render("Enter password:") + "\n\n")
		m.passwordInput.Width = inputWidth
		b.WriteString(m.passwordInput.View())
		pwLen := len(m.passwordInput.Value())
		if pwLen > 0 {
			b.WriteString("\n")
			b.WriteString(Success.Render("✓ " + strings.Repeat("●", pwLen) + " (" + fmt.Sprintf("%d", pwLen) + " chars)"))
		}
		b.WriteString("\n\n")
		b.WriteString(Muted.Render("Enter or Space to decrypt • Esc to cancel"))

	case stepResult, stepExport:
		contentBoxWidth := boxWidth - 8
		if contentBoxWidth < 40 {
			contentBoxWidth = 40
		}
		if m.step == stepExport {
			b.WriteString(Success.Render("✓ Secret unlocked.\n\n"))
			b.WriteString(Box.Width(contentBoxWidth).Render(string(m.plaintext)))
			b.WriteString("\n\n")
			b.WriteString(Prompt.Render("Export path for .env:") + "\n")
			m.pathInput.Width = inputWidth
			b.WriteString(m.pathInput.View())
			b.WriteString("\n")
			b.WriteString(Muted.Render("Enter to save • Esc to go back"))
		} else {
			b.WriteString(Success.Render("✓ Secret unlocked.\n\n"))
			b.WriteString(Box.Width(contentBoxWidth).Render(string(m.plaintext)))
			b.WriteString("\n\n")
			b.WriteString(Prompt.Render("Press ") + "c" + Prompt.Render(" to copy • ") + "e" + Prompt.Render(" to export to .env"))
			b.WriteString("\n")
			b.WriteString(Muted.Render("Esc to go back"))
		}

	case stepBurned:
		b.WriteString(Burn.Render("🔥 Secret burned. One-time use complete."))
		b.WriteString("\n\n")
		b.WriteString(Muted.Render("Enter or Esc to go back"))
	}

	content := Box.Width(boxWidth).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
