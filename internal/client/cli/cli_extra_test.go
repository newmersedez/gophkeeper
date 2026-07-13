package cli_test

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/client/cli"
	"gophkeeper/internal/server/handlers"
	"gophkeeper/internal/server/storage"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIExtendedCommands(t *testing.T) {
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
	require.Equal(t, 0, run("add", "otp", "-title", "otp", "-secret", "JBSWY3DPEHPK3PXP"), errBuf.String())

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

	// find otp id
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
