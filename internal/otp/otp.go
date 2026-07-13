// Package otp реализует генерацию TOTP-кодов (RFC 6238).
package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const (
	defaultPeriod = 30
	defaultDigits = 6
)

// Generate возвращает текущий TOTP-код для base32-секрета.
func Generate(secret string, now time.Time, period uint, digits int) (string, error) {
	if period == 0 {
		period = defaultPeriod
	}
	if digits == 0 {
		digits = defaultDigits
	}

	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}

	counter := uint64(now.Unix()) / uint64(period)
	return hotp(key, counter, digits), nil
}

// Verify проверяет код с допуском ±1 период.
func Verify(secret, code string, now time.Time, period uint, digits int) (bool, error) {
	if period == 0 {
		period = defaultPeriod
	}
	for _, delta := range []int64{-1, 0, 1} {
		t := now.Add(time.Duration(delta) * time.Duration(period) * time.Second)
		got, err := Generate(secret, t, period, digits)
		if err != nil {
			return false, err
		}
		if got == code {
			return true, nil
		}
	}
	return false, nil
}

func decodeSecret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil {
		key, err = base32.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid otp secret: %w", err)
		}
	}
	return key, nil
}

func hotp(key []byte, counter uint64, digits int) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	code := value % mod
	format := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(format, code)
}
