package tui

import (
	"testing"

	"gophkeeper/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestModelViewAndUpdate(t *testing.T) {
	t.Parallel()

	m := model{items: []domain.LocalItem{{
		ID: uuid.New(),
		Payload: domain.ItemPayload{
			Type:  domain.ItemText,
			Title: "note",
			Metadata: map[string]string{"k": "v"},
		},
	}}}

	view := m.View()
	assert.Contains(t, view, "GophKeeper")
	assert.Contains(t, view, "note")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(model)
	assert.Equal(t, 0, m.cursor)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(model)

	empty := model{}
	assert.Contains(t, empty.View(), "Нет записей")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updated.(model)
	assert.True(t, m.quitting)
	assert.NotNil(t, cmd)
	assert.Equal(t, "", m.View())

	_ = m.Init()
}

func TestModelNavigation(t *testing.T) {
	t.Parallel()

	m := model{items: []domain.LocalItem{
		{ID: uuid.New(), Payload: domain.ItemPayload{Type: domain.ItemText, Title: "a"}},
		{ID: uuid.New(), Payload: domain.ItemPayload{Type: domain.ItemText, Title: "b"}},
	}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	assert.Equal(t, 1, m.cursor)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(model)
	assert.Equal(t, 0, m.cursor)
}
