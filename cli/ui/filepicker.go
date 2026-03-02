package ui

import (
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileSelectedMsg is emitted when the user confirms a file in the picker.
type FileSelectedMsg struct {
	Path string
}

type FilePickerModel struct {
	filepicker filepicker.Model
}

func NewFilePicker() FilePickerModel {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".csv"}
	fp.ShowHidden = false
	fp.Height = 20
	if home, err := os.UserHomeDir(); err == nil {
		fp.CurrentDirectory = home
	}
	return FilePickerModel{filepicker: fp}
}

func (m FilePickerModel) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m FilePickerModel) Update(msg tea.Msg) (FilePickerModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		return m, func() tea.Msg { return FileSelectedMsg{Path: path} }
	}

	return m, cmd
}

func (m FilePickerModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))
	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	return titleStyle.Render("GliderCSV") + "\n" +
		subtitleStyle.Render("Select a CSV file — ↑↓ navigate  Enter select  q quit") + "\n\n" +
		m.filepicker.View()
}