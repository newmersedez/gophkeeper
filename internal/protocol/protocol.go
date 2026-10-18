// Package protocol описывает бинарный протокол синхронизации (encoding/gob).
package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func init() {
	gob.Register(SyncRequest{})
	gob.Register(SyncResponse{})
	gob.Register(ItemEnvelope{})
}

// ItemEnvelope — запись сейфа в бинарном протоколе.
type ItemEnvelope struct {
	ID        uuid.UUID
	Version   int64
	UpdatedAt time.Time
	Deleted   bool
	Payload   []byte
}

// SyncRequest — запрос синхронизации клиента.
type SyncRequest struct {
	Since time.Time
	Items []ItemEnvelope
}

// SyncResponse — ответ сервера на синхронизацию.
type SyncResponse struct {
	ServerTime time.Time
	Items      []ItemEnvelope
}

// Encode сериализует значение в gob-байты.
func Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, fmt.Errorf("gob encode: %w", err)
	}
	return buf.Bytes(), nil
}

// Decode десериализует gob-байты в value.
func Decode(data []byte, v any) error {
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(v); err != nil {
		return fmt.Errorf("gob decode: %w", err)
	}
	return nil
}
