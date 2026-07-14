package cli_test

import (
	"bytes"
	"encoding/base32"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/client/cli"
	"gophkeeper/internal/server/handlers"
	"gophkeeper/internal/server/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIVersionAndFlow(t *testing.T) {
	srvStore := storage.OpenTest(t)

	router := handlers.NewRouter(srvStore, auth.NewService("s"), slog.Default())
	server := httptest.NewServer(router.Routes())
	t.Cleanup(server.Close)

	dataDir := t.TempDir()
	info := cli.VersionInfo{Version: "1.2.3", BuildDate: "2026-01-01"}

	var out, errBuf bytes.Buffer
	code := cli.Run([]string{"version"}, &out, &errBuf, info)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "1.2.3")

	out.Reset()
	code = cli.Run([]string{"help"}, &out, &errBuf, info)
	assert.Equal(t, 0, code)

	out.Reset()
	errBuf.Reset()
	code = cli.Run([]string{
		"register", "-server", server.URL, "-data", dataDir,
		"-login", "dave", "-password", "secret",
	}, &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())

	out.Reset()
	errBuf.Reset()
	code = cli.Run([]string{
		"add", "text", "-server", server.URL, "-data", dataDir,
		"-title", "hello", "-content", "world",
	}, &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())

	out.Reset()
	errBuf.Reset()
	code = cli.Run([]string{"list", "-server", server.URL, "-data", dataDir}, &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())
	assert.Contains(t, out.String(), "hello")

	out.Reset()
	errBuf.Reset()
	code = cli.Run([]string{"sync", "-server", server.URL, "-data", dataDir}, &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())

	out.Reset()
	errBuf.Reset()
	code = cli.Run([]string{"sync", "-binary", "-server", server.URL, "-data", dataDir}, &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())
}

func TestCLICommands(t *testing.T) {
	srvStore := storage.OpenTest(t)

	server := httptest.NewServer(handlers.NewRouter(srvStore, auth.NewService("s"), slog.Default()).Routes())
	t.Cleanup(server.Close)

	dataDir := t.TempDir()
	info := cli.VersionInfo{Version: "test", BuildDate: "now"}
	base := []string{"-server", server.URL, "-data", dataDir}

	var out, errBuf bytes.Buffer
	run := func(args ...string) int {
		out.Reset()
		errBuf.Reset()
		return cli.Run(append(args[:1], append(base, args[1:]...)...), &out, &errBuf, info)
	}

	require.Equal(t, 0, run("register", "-login", "u1", "-password", "p1"), errBuf.String())
	require.Equal(t, 0, run("login", "-login", "u1", "-password", "p1"), errBuf.String())

	require.Equal(t, 0, run("add", "credentials", "-title", "gh", "-login", "l", "-password", "p", "-meta", "site=github.com"), errBuf.String())
	require.Equal(t, 0, run("add", "card", "-title", "visa", "-number", "4111", "-holder", "A", "-expiry", "12/30", "-cvv", "123"), errBuf.String())
	// ggignore — demo TOTP seed ("Hello!"), not a real secret
	otpSecret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("Hello!"))
	require.Equal(t, 0, run("add", "otp", "-title", "otp", "-secret", otpSecret), errBuf.String())

	binFile := filepath.Join(t.TempDir(), "blob.bin")
	require.NoError(t, os.WriteFile(binFile, []byte{1, 2, 3}, 0o600))
	require.Equal(t, 0, run("add", "binary", "-title", "bin", "-file", binFile), errBuf.String())

	out.Reset()
	errBuf.Reset()
	code := cli.Run(append([]string{"list"}, base...), &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.NotEmpty(t, lines)
	id := strings.Fields(lines[0])[0]

	out.Reset()
	errBuf.Reset()
	code = cli.Run(append([]string{"get", id}, base...), &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())
	assert.Contains(t, out.String(), "title")

	out.Reset()
	errBuf.Reset()
	_ = cli.Run(append([]string{"list"}, base...), &out, &errBuf, info)
	var otpID string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, "\totp\t") {
			otpID = strings.Fields(line)[0]
			break
		}
	}
	require.NotEmpty(t, otpID)
	out.Reset()
	errBuf.Reset()
	code = cli.Run(append([]string{"otp", otpID}, base...), &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())
	assert.Len(t, strings.TrimSpace(out.String()), 6)

	out.Reset()
	errBuf.Reset()
	code = cli.Run(append([]string{"delete", id}, base...), &out, &errBuf, info)
	require.Equal(t, 0, code, errBuf.String())

	assert.Equal(t, 1, cli.Run([]string{"unknown"}, &out, &errBuf, info))
	assert.Equal(t, 1, cli.Run(nil, &out, &errBuf, info))
}
