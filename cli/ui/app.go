package ui

import (
	"cli/crawler"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenFilePicker screen = iota
	screenTable
)

// AppModel is the root BubbleTea model. It owns screen transitions and
// delegates all updates and rendering to the active sub-model.
type AppModel struct {
	active     screen
	filePicker FilePickerModel
	table      TableModel
	width      int
	height     int
}

func NewApp() AppModel {
	return AppModel{
		active:     screenFilePicker,
		filePicker: NewFilePicker(),
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.filePicker.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Track terminal dimensions at the root so every screen can inherit them.
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
	}

	// FileSelectedMsg triggers the screen transition from picker → table.
	if sel, ok := msg.(FileSelectedMsg); ok {
		client, err := crawler.Open(sel.Path)
		if err != nil {
			// Surface open errors directly in the table screen.
			tm := NewTableModel(nil, sel.Path)
			tm.err = fmt.Errorf("failed to open %s: %w", sel.Path, err)
			tm.loading = false
			m.table = tm
			m.active = screenTable
			return m, nil
		}

		m.table = NewTableModel(client, sel.Path)
		m.active = screenTable

		// Propagate current window size before Init so buildTable has dimensions.
		tableAfterSize, sizeCmd := m.table.Update(tea.WindowSizeMsg{
			Width:  m.width,
			Height: m.height,
		})
		m.table = tableAfterSize

		// Init fires the first NextBatch background worker.
		return m, tea.Batch(m.table.Init(), sizeCmd)
	}

	// Delegate to the active screen.
	switch m.active {
	case screenFilePicker:
		updated, cmd := m.filePicker.Update(msg)
		m.filePicker = updated
		return m, cmd
	case screenTable:
		updated, cmd := m.table.Update(msg)
		m.table = updated
		return m, cmd
	}

	return m, nil
}

func (m AppModel) View() string {
	switch m.active {
	case screenFilePicker:
		return m.filePicker.View()
	case screenTable:
		return m.table.View()
	}
	return ""
}
