package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type menuItem struct {
	title string
	desc  string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type view int

const (
	viewMenu view = iota
	viewRetrieve
	viewSecure
)

type mainModel struct {
	width      int
	height     int
	currentView view
	list       list.Model
	retrieve   *retrieveModel
	secure     *secureModel
}

func newMainModel() mainModel {
	items := []list.Item{
		menuItem{title: "1. Retrieve env", desc: "Paste secure key, enter password, copy or export .env"},
		menuItem{title: "2. Secure env", desc: "Paste secrets, lock with password, get secure key"},
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("205"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("241"))

	// Start with reasonable defaults, will be updated on WindowSizeMsg
	l := list.New(items, delegate, 80, 12)
	l.Title = "🔐 BurnEnv - Zero-Retention Secret Sharing"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)

	return mainModel{
		width:       120,
		height:      30,
		currentView: viewMenu,
		list:        l,
	}
}

func (m mainModel) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Calculate list size based on terminal size
		listWidth := msg.Width - 16
		if listWidth < 50 {
			listWidth = 50
		}
		if listWidth > 100 {
			listWidth = 100
		}
		listHeight := msg.Height - 10
		if listHeight < 8 {
			listHeight = 8
		}
		m.list.SetSize(listWidth, listHeight)
		
		if m.retrieve != nil {
			newRetrieve, _ := m.retrieve.Update(msg)
			if r, ok := newRetrieve.(*retrieveModel); ok {
				m.retrieve = r
			}
		}
		if m.secure != nil {
			newSecure, _ := m.secure.Update(msg)
			if s, ok := newSecure.(*secureModel); ok {
				m.secure = s
			}
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.currentView != viewMenu {
				// Back to menu
				m.currentView = viewMenu
				m.retrieve = nil
				m.secure = nil
				return m, nil
			}
			return m, tea.Quit
		}
	}

	// Delegate to current view
	switch m.currentView {
	case viewMenu:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if cmd != nil {
			return m, cmd
		}
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			i, ok := m.list.SelectedItem().(menuItem)
			if !ok {
				return m, nil
			}
			if strings.Contains(i.title, "1") {
				m.currentView = viewRetrieve
				r := newRetrieveModel(m.width, m.height)
				m.retrieve = &r
				return m, m.retrieve.Init()
			}
			if strings.Contains(i.title, "2") {
				m.currentView = viewSecure
				s := newSecureModel(m.width, m.height)
				m.secure = &s
				return m, m.secure.Init()
			}
		}
		return m, nil

	case viewRetrieve:
		newRetrieve, cmd := m.retrieve.Update(msg)
		if r, ok := newRetrieve.(*retrieveModel); ok {
			m.retrieve = r
			if r.done {
				m.currentView = viewMenu
				m.retrieve = nil
			}
		}
		return m, cmd

	case viewSecure:
		newSecure, cmd := m.secure.Update(msg)
		if s, ok := newSecure.(*secureModel); ok {
			m.secure = s
			if s.done {
				m.currentView = viewMenu
				m.secure = nil
			}
		}
		return m, cmd
	}

	return m, nil
}

func (m mainModel) View() string {
	// Use most of the terminal width
	boxWidth := m.width - 8
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 120 {
		boxWidth = 120
	}

	var content string
	switch m.currentView {
	case viewMenu:
		// Wrap menu in a styled box
		menuContent := m.list.View()
		content = Box.Width(boxWidth).Render(menuContent)
	case viewRetrieve:
		content = m.retrieve.View()
	case viewSecure:
		content = m.secure.View()
	default:
		content = ""
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// RunMainTUI launches the main BurnEnv TUI (menu + retrieve + secure flows).
func RunMainTUI() error {
	m := newMainModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
