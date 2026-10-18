package domain_test

import (
	"testing"

	"gophkeeper/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestItemTypeValid(t *testing.T) {
	t.Parallel()

	assert.True(t, domain.ItemCredentials.Valid())
	assert.True(t, domain.ItemText.Valid())
	assert.True(t, domain.ItemBinary.Valid())
	assert.True(t, domain.ItemBankCard.Valid())
	assert.True(t, domain.ItemOTP.Valid())
	assert.False(t, domain.ItemType("unknown").Valid())
}
