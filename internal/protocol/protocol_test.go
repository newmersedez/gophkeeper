package protocol_test

import (
	"testing"
	"time"

	"gophkeeper/internal/protocol"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	t.Parallel()

	req := protocol.SyncRequest{
		Since: time.Unix(100, 0).UTC(),
		Items: []protocol.ItemEnvelope{{
			ID: uuid.New(), Version: 3, UpdatedAt: time.Now().UTC(), Payload: []byte("abc"),
		}},
	}
	data, err := protocol.Encode(req)
	require.NoError(t, err)

	var out protocol.SyncRequest
	require.NoError(t, protocol.Decode(data, &out))
	assert.Equal(t, req.Since, out.Since)
	require.Len(t, out.Items, 1)
	assert.Equal(t, req.Items[0].Payload, out.Items[0].Payload)
}
