package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yesahem/burnenv/internal/client"
	"github.com/yesahem/burnenv/internal/crypto"
	"github.com/yesahem/burnenv/internal/store"
)

type createResult struct {
	link string
	err  error
}

type createModel struct {
	secretInput   textinput.Model
	passwordInput textinput.Model
	expiry        int
	maxViews      int
	serverURL     string
	focused       int
	secret        string
	password      string
	result        *createResult
	quitting      bool
	width         int
	height        int
}

func newCreateModel(expiry, maxViews int, serverURL string) createModel {
	si := textinput.New()
	si.Placeholder = "Paste or type your secret..."
	si.Width = 72

	pi := textinput.New()
	pi.Placeholder = "••••••••"
	pi.EchoMode = textinput.EchoPassword
	pi.Width = 72

	return createModel{
		secretInput:   si,
		passwordInput: pi,
		expiry:        expiry,
		maxViews:      maxViews,
		serverURL:     serverURL,
		focused:       0,
		width:         80,
		height:        24,
	}
}

func (m createModel) inputWidth() int {
	w := m.width - 8
	if w < 40 {
		w = 40
	}
	return w
}

func (m createModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.secretInput.Width = m.inputWidth()
		m.passwordInput.Width = m.inputWidth()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "tab", "enter":
			if m.result != nil {
				return m, tea.Quit
			}
			if m.focused == 0 {
				m.secret = m.secretInput.Value()
				m.focused = 1
				m.secretInput.Blur()
				m.passwordInput.Focus()
				return m, textinput.Blink
			}
			if m.focused == 1 {
				m.password = m.passwordInput.Value()
				m.focused = 2
				m.passwordInput.Blur()
				return m, m.doCreateSecret
			}
		}
	case createResult:
		m.result = &msg
		return m, nil
	}

	var cmd tea.Cmd
	if m.focused == 0 {
		m.secretInput, cmd = m.secretInput.Update(msg)
	} else if m.focused == 1 {
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}
	return m, cmd
}

func (m createModel) doCreateSecret() tea.Msg {
	secret := m.secret
	if secret == "" {
		return createResult{err: fmt.Errorf("secret cannot be empty")}
	}
	if m.password == "" {
		return createResult{err: fmt.Errorf("password cannot be empty")}
	}

	payload, err := crypto.Encrypt([]byte(secret), m.password)
	if err != nil {
		return createResult{err: err}
	}

	expiry := time.Now().Add(time.Duration(m.expiry) * time.Minute).Unix()
	payload.Expiry = expiry
	payload.MaxViews = m.maxViews

	url := m.serverURL
	if url == "" {
		url = os.Getenv("BURNENV_SERVER")
	}

	var link string
	if url != "" {
		link, err = client.Create(url, payload)
	} else {
		mock, err2 := store.NewMockStore("")
		if err2 != nil {
			return createResult{err: err2}
		}
		link, err = mock.Save(payload)
	}
	if err != nil {
		return createResult{err: err}
	}
	return createResult{link: link}
}

func (m createModel) View() string {
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
			b.WriteString(Success.Render("✓ Secret encrypted.\n\n"))
			b.WriteString("Burn link:\n")
			b.WriteString(Link.Render(m.result.link) + "\n\n")
			b.WriteString(Muted.Render(fmt.Sprintf("Expires: %d min | Max views: %d", m.expiry, m.maxViews)))
			content = Box.Width(boxWidth).Render(b.String())
		}
	} else {
		var b strings.Builder
		b.WriteString(Title.Render("BurnEnv Create") + "\n\n")

		if m.focused == 0 {
			b.WriteString(Focused.Render("> Secret:") + "\n")
			b.WriteString(m.secretInput.View() + "\n\n")
			b.WriteString(Muted.Render("  Password: (next)") + "\n\n")
		} else {
			b.WriteString(Prompt.Render("  Secret: [entered]") + "\n\n")
			b.WriteString(Focused.Render("> Password:") + "\n")
			b.WriteString(m.passwordInput.View() + "\n\n")
		}

		b.WriteString(Muted.Render("Tab/Enter to continue • Ctrl+C to cancel"))
		content = Box.Width(boxWidth).Render(b.String())
	}

	// Center content in full terminal viewport
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// RunCreateTUI launches the Bubble Tea TUI for create.
func RunCreateTUI(expiry, maxViews int, serverURL string) (string, error) {
	m := newCreateModel(expiry, maxViews, serverURL)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	fm := final.(createModel)
	if fm.result != nil && fm.result.err != nil {
		return "", fm.result.err
	}
	if fm.result != nil {
		return fm.result.link, nil
	}
	return "", fmt.Errorf("cancelled")
}
