// Package domain содержит доменные модели GophKeeper.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ItemType описывает тип хранимой записи.
type ItemType string

const (
	// ItemCredentials — пара логин/пароль.
	ItemCredentials ItemType = "credentials"
	// ItemText — произвольный текст.
	ItemText ItemType = "text"
	// ItemBinary — произвольные бинарные данные.
	ItemBinary ItemType = "binary"
	// ItemBankCard — данные банковской карты.
	ItemBankCard ItemType = "bank_card"
	// ItemOTP — секрет для TOTP/HOTP.
	ItemOTP ItemType = "otp"
)

// Valid сообщает, является ли тип поддерживаемым.
func (t ItemType) Valid() bool {
	switch t {
	case ItemCredentials, ItemText, ItemBinary, ItemBankCard, ItemOTP:
		return true
	default:
		return false
	}
}

// User представляет учётную запись.
type User struct {
	ID           uuid.UUID
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

// VaultItem — запись сейфа на сервере (полезная нагрузка уже зашифрована клиентом).
type VaultItem struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Version   int64
	UpdatedAt time.Time
	Deleted   bool
	Payload   []byte
}

// CredentialsData — полезные данные типа credentials (до шифрования).
type CredentialsData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// TextData — произвольный текст.
type TextData struct {
	Content string `json:"content"`
}

// BinaryData — бинарные данные в base64-представлении для JSON.
type BinaryData struct {
	Content []byte `json:"content"`
	Mime    string `json:"mime,omitempty"`
}

// BankCardData — данные банковской карты.
type BankCardData struct {
	Number     string `json:"number"`
	Holder     string `json:"holder"`
	Expiry     string `json:"expiry"`
	CVV        string `json:"cvv"`
	Bank       string `json:"bank,omitempty"`
}

// OTPData — секрет одноразовых паролей.
type OTPData struct {
	Secret  string `json:"secret"`
	Issuer  string `json:"issuer,omitempty"`
	Account string `json:"account,omitempty"`
	Period  uint   `json:"period,omitempty"`
	Digits  int    `json:"digits,omitempty"`
}

// ItemPayload — расшифрованное содержимое записи сейфа.
type ItemPayload struct {
	Type     ItemType               `json:"type"`
	Title    string                 `json:"title"`
	Metadata map[string]string      `json:"metadata,omitempty"`
	Credentials *CredentialsData    `json:"credentials,omitempty"`
	Text        *TextData           `json:"text,omitempty"`
	Binary      *BinaryData         `json:"binary,omitempty"`
	BankCard    *BankCardData       `json:"bank_card,omitempty"`
	OTP         *OTPData            `json:"otp,omitempty"`
}

// LocalItem — локальная копия записи на клиенте.
type LocalItem struct {
	ID        uuid.UUID
	Version   int64
	UpdatedAt time.Time
	Deleted   bool
	Dirty     bool
	Payload   ItemPayload
}
