package ui

import (
	"cli/crawler"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// lazyLoadThreshold is the number of rows from the end of loaded data at which
// a new batch is requested.  Keeping it comfortably larger than one batch size
// (100) means the user never reaches the end of available rows while waiting.
const lazyLoadThreshold = 150

var tableContainerStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

// loadBatchCmd calls NextBatch in a background goroutine and returns a message.
// This must never block the BubbleTea event loop.
func loadBatchCmd(c *crawler.Client) tea.Cmd {
	return func() tea.Msg {
		header, rows, err := c.NextBatch()
		if err == io.EOF {
			return EOFMsg{}
		}
		if err != nil {
			return CrawlerErrorMsg{Err: err}
		}
		return BatchLoadedMsg{Header: header, Rows: rows}
	}
}

// TableModel is the CSV table screen.  It supports virtual scrolling: only a
// single viewport-sized slice of allRows is handed to the bubbles table at any
// time.  New batches are fetched lazily whenever the cursor approaches the end
// of the currently loaded data.
type TableModel struct {
	client     *crawler.Client
	table      table.Model
	header     []string
	allRows    [][]string
	viewOffset int // absolute index of the first row shown in the viewport
	cursorRow  int // absolute index of the selected row
	loading    bool
	done       bool
	err        error
	width      int
	height     int
	filePath   string
}

func NewTableModel(client *crawler.Client, filePath string) TableModel {
	return TableModel{
		client:   client,
		loading:  true,
		filePath: filePath,
	}
}

func (m TableModel) Init() tea.Cmd {
	return loadBatchCmd(m.client)
}

// viewportHeight returns the number of data rows that fit on screen.
func (m TableModel) viewportHeight() int {
	return max(m.height-6, 5)
}

// needsMoreData reports whether the cursor is close enough to the end of
// loaded data that another batch should be requested.
func (m TableModel) needsMoreData() bool {
	if m.done || m.loading || m.client == nil {
		return false
	}
	return len(m.allRows) == 0 || m.cursorRow >= len(m.allRows)-lazyLoadThreshold
}

func (m TableModel) Update(msg tea.Msg) (TableModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if len(m.header) > 0 {
			m.table = m.buildTable()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.client != nil {
				m.client.Close()
			}
			return m, tea.Quit

		case "down", "j":
			if m.cursorRow < len(m.allRows)-1 {
				m.cursorRow++
				vpH := m.viewportHeight()
				if m.cursorRow >= m.viewOffset+vpH {
					m.viewOffset = m.cursorRow - vpH + 1
				}
				m.table = m.buildTable()
			}
			if m.needsMoreData() {
				m.loading = true
				return m, loadBatchCmd(m.client)
			}
			return m, nil

		case "up", "k":
			if m.cursorRow > 0 {
				m.cursorRow--
				if m.cursorRow < m.viewOffset {
					m.viewOffset = m.cursorRow
				}
				m.table = m.buildTable()
			}
			return m, nil

		case "pgdown", "ctrl+f":
			vpH := m.viewportHeight()
			m.cursorRow = min(m.cursorRow+vpH, max(len(m.allRows)-1, 0))
			if m.cursorRow >= m.viewOffset+vpH {
				m.viewOffset = m.cursorRow - vpH + 1
			}
			m.table = m.buildTable()
			if m.needsMoreData() {
				m.loading = true
				return m, loadBatchCmd(m.client)
			}
			return m, nil

		case "pgup", "ctrl+b":
			vpH := m.viewportHeight()
			m.cursorRow = max(m.cursorRow-vpH, 0)
			if m.cursorRow < m.viewOffset {
				m.viewOffset = m.cursorRow
			}
			m.table = m.buildTable()
			return m, nil

		case "g", "home":
			m.cursorRow = 0
			m.viewOffset = 0
			m.table = m.buildTable()
			return m, nil

		case "G", "end":
			m.cursorRow = max(len(m.allRows)-1, 0)
			vpH := m.viewportHeight()
			m.viewOffset = max(m.cursorRow-vpH+1, 0)
			m.table = m.buildTable()
			if m.needsMoreData() {
				m.loading = true
				return m, loadBatchCmd(m.client)
			}
			return m, nil
		}

	case BatchLoadedMsg:
		if len(m.header) == 0 {
			m.header = msg.Header
		}
		m.allRows = append(m.allRows, msg.Rows...)
		m.loading = false
		m.table = m.buildTable()
		// If the cursor is still near the end after this batch (e.g. the user
		// jumped to G), keep fetching.
		if m.needsMoreData() {
			m.loading = true
			return m, loadBatchCmd(m.client)
		}
		return m, nil

	case EOFMsg:
		m.done = true
		m.loading = false
		m.table = m.buildTable()
		return m, nil

	case CrawlerErrorMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil
	}

	return m, nil
}

// buildTable constructs a bubbles table containing only the current viewport
// slice of allRows.  The table cursor is set to the relative position of
// cursorRow within that slice.
func (m TableModel) buildTable() table.Model {
	if len(m.header) == 0 {
		return table.New()
	}

	colWidth := min(max(12, (m.width-4)/len(m.header)), 32)

	cols := make([]table.Column, len(m.header))
	for i, h := range m.header {
		w := max(len(h)+2, colWidth)
		cols[i] = table.Column{Title: h, Width: w}
	}

	// Slice only the visible window.
	vpH := m.viewportHeight()
	end := min(m.viewOffset+vpH, len(m.allRows))
	viewSlice := m.allRows[m.viewOffset:end]

	rows := make([]table.Row, len(viewSlice))
	for i, r := range viewSlice {
		rows[i] = table.Row(r)
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(vpH),
	)

	// Keep the table's own cursor in sync with our absolute cursor.
	if rel := m.cursorRow - m.viewOffset; rel >= 0 && rel < len(rows) {
		t.SetCursor(rel)
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)
	return t
}

func (m TableModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).
			Render(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	header := titleStyle.Render("GliderCSV") + "  " + pathStyle.Render(m.filePath)

	rowNum := m.cursorRow + 1
	total := len(m.allRows)
	var status string
	if m.loading {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).
			Render(fmt.Sprintf("  row %d / %d+ (loading…)", rowNum, total))
	} else {
		suffix := ""
		if !m.done {
			suffix = "+"
		}
		status = pathStyle.Render(fmt.Sprintf(
			"  row %d / %d%s  ↑↓/jk  pgup/pgdn  g/G  q quit",
			rowNum, total, suffix,
		))
	}

	return header + status + "\n" + tableContainerStyle.Render(m.table.View())
}
