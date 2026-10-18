// Package cli реализует команды CLI-клиента GophKeeper.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gophkeeper/internal/client"
	"gophkeeper/internal/client/app"
	"gophkeeper/internal/client/localstore"
	"gophkeeper/internal/client/tui"
	"gophkeeper/internal/domain"

	"github.com/google/uuid"
)

// VersionInfo содержит метаданные сборки клиента.
type VersionInfo struct {
	Version   string
	BuildDate string
}

// Run разбирает аргументы CLI и выполняет команду. Возвращает код выхода.
func Run(args []string, stdout, stderr io.Writer, info VersionInfo) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 1
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "version", "-version", "--version":
		fmt.Fprintf(stdout, "gophkeeper %s (built %s)\n", info.Version, info.BuildDate)
		return 0
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	}

	application, cleanup, err := newApp(rest)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer cleanup()

	// newApp consumes global flags from rest via a FlagSet; remaining args are command-specific.
	// We re-parse by filtering known global flags.
	cmdArgs := stripGlobalFlags(rest)

	ctx := context.Background()
	switch cmd {
	case "register":
		return runRegister(ctx, application, cmdArgs, stdout, stderr)
	case "login":
		return runLogin(ctx, application, cmdArgs, stdout, stderr)
	case "add":
		return runAdd(application, cmdArgs, stdout, stderr)
	case "list":
		return runList(application, stdout, stderr)
	case "get":
		return runGet(application, cmdArgs, stdout, stderr)
	case "delete":
		return runDelete(application, cmdArgs, stdout, stderr)
	case "sync":
		return runSync(ctx, application, cmdArgs, stdout, stderr)
	case "otp":
		return runOTP(application, cmdArgs, stdout, stderr)
	case "tui":
		if err := application.RestoreSession(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		items, err := application.ListItems()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := tui.Run(items); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `GophKeeper CLI

Usage:
  gophkeeper <command> [flags]

Commands:
  register   Register a new user
  login      Authenticate and store session
  add        Add vault item (credentials|text|binary|card|otp)
  list       List local vault items
  get        Show vault item by id
  delete     Soft-delete vault item
  sync       Synchronize with server (--binary for gob protocol)
  otp        Show current TOTP code for otp item
  tui        Open terminal UI
  version    Print version and build date

Global flags:
  -server    Server base URL (default http://localhost:8080)
  -data      Client data directory (default ~/.gophkeeper)
`)
}

func newApp(args []string) (*app.App, func(), error) {
	fs := flag.NewFlagSet("gophkeeper", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	server := fs.String("server", envOr("GOPHKEEPER_SERVER", "http://localhost:8080"), "server URL")
	dataDir := fs.String("data", "", "client data directory")
	_ = fs.Parse(filterGlobalFlags(args))

	dir := *dataDir
	if dir == "" {
		var err error
		dir, err = app.DefaultDataDir()
		if err != nil {
			return nil, nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, err
	}

	store, err := localstore.Open(filepath.Join(dir, "vault.db"))
	if err != nil {
		return nil, nil, err
	}
	api := client.New(*server, nil)
	application := app.New(api, store, dir)
	return application, func() { _ = store.Close() }, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func filterGlobalFlags(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-server" || a == "-data":
			out = append(out, a)
			if i+1 < len(args) {
				i++
				out = append(out, args[i])
			}
		case strings.HasPrefix(a, "-server=") || strings.HasPrefix(a, "-data="):
			out = append(out, a)
		}
	}
	return out
}

func stripGlobalFlags(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-server" || a == "-data":
			i++
		case strings.HasPrefix(a, "-server=") || strings.HasPrefix(a, "-data="):
			// skip
		default:
			out = append(out, a)
		}
	}
	return out
}

func runRegister(ctx context.Context, a *app.App, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(stderr)
	login := fs.String("login", "", "login")
	password := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *login == "" || *password == "" {
		fmt.Fprintln(stderr, "login and password are required")
		return 1
	}
	if err := a.Register(ctx, *login, *password); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "registered")
	return 0
}

func runLogin(ctx context.Context, a *app.App, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	login := fs.String("login", "", "login")
	password := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *login == "" || *password == "" {
		fmt.Fprintln(stderr, "login and password are required")
		return 1
	}
	if err := a.Login(ctx, *login, *password); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "logged in")
	return 0
}

func runAdd(a *app.App, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: add <credentials|text|binary|card|otp> [flags]")
		return 1
	}
	if err := a.RestoreSession(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	kind := args[0]
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	title := fs.String("title", "", "title")
	meta := fs.String("meta", "", "metadata as key=value,key2=value2")

	var payload domain.ItemPayload
	switch kind {
	case "credentials":
		login := fs.String("login", "", "login")
		password := fs.String("password", "", "password")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		payload = domain.ItemPayload{
			Type: domain.ItemCredentials, Title: *title,
			Credentials: &domain.CredentialsData{Login: *login, Password: *password},
		}
	case "text":
		content := fs.String("content", "", "text content")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		payload = domain.ItemPayload{
			Type: domain.ItemText, Title: *title,
			Text: &domain.TextData{Content: *content},
		}
	case "binary":
		file := fs.String("file", "", "path to file")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		data, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		payload = domain.ItemPayload{
			Type: domain.ItemBinary, Title: *title,
			Binary: &domain.BinaryData{Content: data},
		}
	case "card":
		number := fs.String("number", "", "card number")
		holder := fs.String("holder", "", "card holder")
		expiry := fs.String("expiry", "", "expiry MM/YY")
		cvv := fs.String("cvv", "", "cvv")
		bank := fs.String("bank", "", "bank name")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		payload = domain.ItemPayload{
			Type: domain.ItemBankCard, Title: *title,
			BankCard: &domain.BankCardData{Number: *number, Holder: *holder, Expiry: *expiry, CVV: *cvv, Bank: *bank},
		}
	case "otp":
		secret := fs.String("secret", "", "base32 otp secret")
		issuer := fs.String("issuer", "", "issuer")
		account := fs.String("account", "", "account")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		payload = domain.ItemPayload{
			Type: domain.ItemOTP, Title: *title,
			OTP: &domain.OTPData{Secret: *secret, Issuer: *issuer, Account: *account, Period: 30, Digits: 6},
		}
	default:
		fmt.Fprintf(stderr, "unknown item type: %s\n", kind)
		return 1
	}

	if payload.Title == "" {
		fmt.Fprintln(stderr, "title is required")
		return 1
	}
	payload.Metadata = parseMeta(*meta)

	item, err := a.AddItem(payload)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "added %s\n", item.ID)
	return 0
}

func parseMeta(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out
}

func runList(a *app.App, stdout, stderr io.Writer) int {
	if err := a.RestoreSession(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	items, err := a.ListItems()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, it := range items {
		fmt.Fprintf(stdout, "%s\t%s\t%s\tv%d\n", it.ID, it.Payload.Type, it.Payload.Title, it.Version)
	}
	return 0
}

func runGet(a *app.App, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: get <id>")
		return 1
	}
	if err := a.RestoreSession(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	id, err := uuid.Parse(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "invalid id")
		return 1
	}
	item, err := a.GetItem(id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(item)
	return 0
}

func runDelete(a *app.App, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: delete <id>")
		return 1
	}
	if err := a.RestoreSession(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	id, err := uuid.Parse(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "invalid id")
		return 1
	}
	if err := a.DeleteItem(id); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "deleted")
	return 0
}

func runSync(ctx context.Context, a *app.App, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	binary := fs.Bool("binary", false, "use binary gob protocol")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if err := a.Sync(ctx, *binary); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "synced at %s\n", time.Now().UTC().Format(time.RFC3339))
	return 0
}

func runOTP(a *app.App, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: otp <id>")
		return 1
	}
	if err := a.RestoreSession(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	id, err := uuid.Parse(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "invalid id")
		return 1
	}
	code, err := a.OTPCode(id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, code)
	return 0
}
