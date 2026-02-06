package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yesahem/burnenv/internal/client"
	"github.com/yesahem/burnenv/internal/crypto"
	"github.com/yesahem/burnenv/internal/store"
)

type secureStep int

const (
	secStepSecrets secureStep = iota
	secStepPassword
	secStepMaxViews
	secStepResult
)

type secureModel struct {
	width         int
	height        int
	step          secureStep
	secretInput   textarea.Model
	passwordInput textinput.Model
	maxViews      int
	maxViewsIdx   int
	secrets       string
	password      string
	secureKey     string
	err           error
	done          bool
	serverURL     string
}

func newSecureModel(width, height int) secureModel {
	si := textarea.New()
	si.Placeholder = "Paste your secrets here (multi-line supported)...\nExample:\nAPI_KEY=abc123\nDB_PASSWORD=secret"
	si.SetWidth(60)
	si.SetHeight(8)
	si.ShowLineNumbers = false
	si.Focus() // Focus immediately so it's ready for input

	pi := textinput.New()
	pi.Placeholder = "••••••••"
	pi.EchoMode = textinput.EchoPassword
	pi.Width = 60

	return secureModel{
		width:         width,
		height:        height,
		step:          secStepSecrets,
		secretInput:   si,
		passwordInput: pi,
		maxViews:      1,
		maxViewsIdx:   0,
		serverURL:     os.Getenv("BURNENV_SERVER"),
	}
}

func (m secureModel) Init() tea.Cmd {
	return textarea.Blink
}

type secureResult struct {
	key string
	err error
}

func (m secureModel) doCreate() tea.Msg {
	secrets := m.secrets
	if secrets == "" {
		return secureResult{err: fmt.Errorf("secrets cannot be empty")}
	}
	if m.password == "" {
		return secureResult{err: fmt.Errorf("password cannot be empty")}
	}

	payload, err := crypto.Encrypt([]byte(secrets), m.password)
	if err != nil {
		return secureResult{err: err}
	}

	expiry := time.Now().Add(5 * time.Minute).Unix()
	payload.Expiry = expiry
	payload.MaxViews = m.maxViews

	var key string
	if m.serverURL != "" {
		key, err = client.Create(m.serverURL, payload)
	} else {
		mock, err2 := store.NewMockStore("")
		if err2 != nil {
			return secureResult{err: err2}
		}
		key, err = mock.Save(payload)
	}
	if err != nil {
		return secureResult{err: err}
	}
	return secureResult{key: key}
}

func (m secureModel) inputWidth() int {
	w := m.width - 12
	if w < 40 {
		w = 40
	}
	return w
}

func (m secureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := m.inputWidth()
		m.secretInput.SetWidth(w)
		m.passwordInput.Width = w
		return &m, nil

	case tea.KeyMsg:
		keyStr := msg.String()

		// Handle global keys first
		switch keyStr {
		case "ctrl+c":
			return &m, tea.Quit
		case "esc":
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

		// Space detection for max-views step
		isSpace := msg.Type == tea.KeySpace || keyStr == " " || keyStr == "space"

		// Handle max-views step FIRST before any input forwarding
		if m.step == secStepMaxViews {
			// Confirm with Enter or Space
			if isEnter || isSpace {
				return &m, m.doCreate
			}
			// Navigation
			switch keyStr {
			case "up", "k":
				if m.maxViewsIdx > 0 {
					m.maxViewsIdx--
					m.maxViews = m.maxViewsIdx + 1
				}
				return &m, nil
			case "down", "j":
				if m.maxViewsIdx < 4 {
					m.maxViewsIdx++
					m.maxViews = m.maxViewsIdx + 1
				}
				return &m, nil
			case "1", "2", "3", "4", "5":
				m.maxViews = int(keyStr[0] - '0')
				m.maxViewsIdx = m.maxViews - 1
				return &m, nil
			}
			// Ctrl+S also confirms
			if keyStr == "ctrl+s" {
				return &m, m.doCreate
			}
			return &m, nil
		}

		// Handle result step
		if m.step == secStepResult {
			if isEnter || isSpace {
				m.done = true
				return &m, nil
			}
			return &m, nil
		}

		// Handle password step - Enter advances
		if m.step == secStepPassword && isEnter {
			m.password = m.passwordInput.Value()
			m.passwordInput.Blur()
			m.step = secStepMaxViews
			return &m, nil
		}

		// Handle secrets step - Ctrl+S or Ctrl+Enter advances
		if m.step == secStepSecrets {
			if keyStr == "ctrl+s" || keyStr == "ctrl+enter" {
				m.secrets = m.secretInput.Value()
				m.secretInput.Blur()
				m.passwordInput.Focus()
				m.step = secStepPassword
				return &m, textinput.Blink
			}
		}

	case secureResult:
		if msg.err != nil {
			m.err = msg.err
			return &m, nil
		}
		m.secureKey = msg.key
		m.step = secStepResult
		return &m, nil
	}

	// Forward to focused input
	if m.step == secStepSecrets {
		var cmd tea.Cmd
		m.secretInput, cmd = m.secretInput.Update(msg)
		return &m, cmd
	}
	if m.step == secStepPassword {
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return &m, cmd
	}

	return &m, nil
}

func (m secureModel) View() string {
	// Use most of the terminal width
	boxWidth := m.width - 8
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 120 {
		boxWidth = 120 // Cap max width for readability
	}

	// Update textarea width to match
	inputWidth := boxWidth - 8
	if inputWidth < 50 {
		inputWidth = 50
	}

	var b strings.Builder

	switch m.step {
	case secStepSecrets:
		b.WriteString(Title.Render("Secure env") + "\n\n")
		b.WriteString(Prompt.Render("Paste or type your secrets below:") + "\n")
		b.WriteString(Muted.Render("(Content is visible for verification)") + "\n\n")
		// Dynamically set width
		m.secretInput.SetWidth(inputWidth)
		b.WriteString(m.secretInput.View())
		secretLen := len(m.secretInput.Value())
		lines := strings.Count(m.secretInput.Value(), "\n") + 1
		if secretLen > 0 {
			b.WriteString("\n")
			b.WriteString(Success.Render(fmt.Sprintf("✓ %d characters, %d line(s)", secretLen, lines)))
		}
		b.WriteString("\n\n")
		b.WriteString(Muted.Render("Ctrl+S to continue • Esc to cancel"))

	case secStepPassword:
		b.WriteString(Title.Render("Secure env") + "\n\n")
		b.WriteString(Prompt.Render("Lock with password:") + "\n")
		b.WriteString(m.passwordInput.View())
		pwLen := len(m.passwordInput.Value())
		if pwLen > 0 {
			b.WriteString("\n")
			b.WriteString(Success.Render("✓ "+strings.Repeat("●", pwLen)+" ("+fmt.Sprintf("%d", pwLen)+" chars)"))
		}
		b.WriteString("\n\n")
		b.WriteString(Muted.Render("Enter to continue • Esc to cancel"))

	case secStepMaxViews:
		b.WriteString(Title.Render("Secure env") + "\n\n")
		b.WriteString(Prompt.Render("Max views (1-5):") + "\n\n")
		for i := 1; i <= 5; i++ {
			marker := "  "
			if i == m.maxViews {
				marker = "> "
			}
			if i == m.maxViews {
				b.WriteString(Focused.Render(marker + fmt.Sprintf("%d person(s) can view", i)) + "\n")
			} else {
				b.WriteString(Prompt.Render(marker + fmt.Sprintf("%d person(s) can view", i)) + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(Muted.Render("↑/↓ or 1-5 to select • Enter or Space to create"))

	case secStepResult:
		if m.err != nil {
			b.WriteString(Error.Render("Error: "+m.err.Error()) + "\n\n")
			b.WriteString(Muted.Render("Esc to go back"))
		} else {
			b.WriteString(Success.Render("✓ Secrets secured.\n\n"))
			b.WriteString("Secure key:\n")
			b.WriteString(Link.Render(m.secureKey) + "\n\n")
			b.WriteString(Muted.Render("Share this key with up to " + fmt.Sprintf("%d", m.maxViews) + " person(s). Enter or Esc to go back"))
		}
	}

	content := Box.Width(boxWidth).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
