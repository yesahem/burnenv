package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Title for headers
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	// Success for positive outcomes
	Success = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	// Error for failures
	Error = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	// Link for URLs
	Link = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	// Muted for secondary info
	Muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	// Burn for destruction events
	Burn = lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true)

	// Input prompt
	Prompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color("99"))

	// Box around content
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(1, 2)

	// Focused input
	Focused = lipgloss.NewStyle().
		Foreground(lipgloss.Color("205"))
)
