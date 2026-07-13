// Package tui предоставляет простой терминальный интерфейс списка записей.
package tui

import (
	"fmt"
	"strings"

	"gophkeeper/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	items  []domain.LocalItem
	cursor int
	quitting bool
}

// Run запускает TUI со списком записей сейфа.
func Run(items []domain.LocalItem) error {
	p := tea.NewProgram(model{items: items})
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString("GophKeeper TUI (q — выход)\n\n")
	if len(m.items) == 0 {
		b.WriteString("Нет записей. Добавьте данные через CLI и выполните sync.\n")
		return b.String()
	}
	for i, it := range m.items {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		fmt.Fprintf(&b, "%s %s  [%s]  %s\n", cursor, it.ID.String()[:8], it.Payload.Type, it.Payload.Title)
	}
	if m.cursor >= 0 && m.cursor < len(m.items) {
		it := m.items[m.cursor]
		b.WriteString("\n---\n")
		fmt.Fprintf(&b, "ID: %s\nType: %s\nTitle: %s\nVersion: %d\n", it.ID, it.Payload.Type, it.Payload.Title, it.Version)
		if len(it.Payload.Metadata) > 0 {
			b.WriteString("Meta:\n")
			for k, v := range it.Payload.Metadata {
				fmt.Fprintf(&b, "  %s=%s\n", k, v)
			}
		}
	}
	return b.String()
}
