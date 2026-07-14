// Package crypto предоставляет клиентское шифрование данных сейфа (AES-256-GCM).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltSize   = 16
	nonceSize  = 12
	keySize    = 32
	iterations = 100_000
)

// ErrDecryptionFailed возвращается при неверном ключе или повреждённых данных.
var ErrDecryptionFailed = errors.New("decryption failed")

// DeriveKey выводит ключ шифрования из пароля и соли (PBKDF2-SHA256).
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, iterations, keySize, sha256.New)
}

// NewSalt генерирует криптостойкую соль.
func NewSalt() ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// Encrypt шифрует plaintext ключом key и возвращает salt||nonce||ciphertext.
// Если salt == nil, создаётся новая соль (для первого шифрования сессии храните соль отдельно).
func Encrypt(plaintext, key, salt []byte) ([]byte, error) {
	if len(key) != keySize {
		return nil, errors.New("invalid key size")
	}
	if salt == nil {
		var err error
		salt, err = NewSalt()
		if err != nil {
			return nil, err
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt расшифровывает данные формата salt||nonce||ciphertext.
// Ключ должен быть получен через DeriveKey(password, salt из blob).
func Decrypt(blob, key []byte) ([]byte, error) {
	if len(blob) < saltSize+nonceSize {
		return nil, ErrDecryptionFailed
	}
	if len(key) != keySize {
		return nil, errors.New("invalid key size")
	}

	nonce := blob[saltSize : saltSize+nonceSize]
	ciphertext := blob[saltSize+nonceSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

// ExtractSalt возвращает соль из зашифрованного blob.
func ExtractSalt(blob []byte) ([]byte, error) {
	if len(blob) < saltSize {
		return nil, ErrDecryptionFailed
	}
	salt := make([]byte, saltSize)
	copy(salt, blob[:saltSize])
	return salt, nil
}

// EncryptWithPassword шифрует данные паролем (соль встраивается в результат).
func EncryptWithPassword(plaintext []byte, password string) ([]byte, error) {
	salt, err := NewSalt()
	if err != nil {
		return nil, err
	}
	key := DeriveKey(password, salt)
	return Encrypt(plaintext, key, salt)
}

// DecryptWithPassword расшифровывает данные, зашифрованные EncryptWithPassword.
func DecryptWithPassword(blob []byte, password string) ([]byte, error) {
	salt, err := ExtractSalt(blob)
	if err != nil {
		return nil, err
	}
	key := DeriveKey(password, salt)
	return Decrypt(blob, key)
}

// EncodeBase64 кодирует байты в base64.
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 декодирует base64.
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
