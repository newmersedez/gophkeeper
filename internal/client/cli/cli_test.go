package cli_test

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"gophkeeper/internal/auth"
	"gophkeeper/internal/client/cli"
	"gophkeeper/internal/server/handlers"
	"gophkeeper/internal/server/storage"

	"log/slog"

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
