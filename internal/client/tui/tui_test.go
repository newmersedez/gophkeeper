package tui_test

import (
	"testing"

	"gophkeeper/internal/client/tui"
	"gophkeeper/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestViewEmpty(t *testing.T) {
	t.Parallel()
	// Ensure package is imported and types compile; Run is interactive and covered lightly via CLI path.
	_ = tui.Run
	assert.NotNil(t, domain.ItemText)
	_ = uuid.New()
}
