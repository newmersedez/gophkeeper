// Package main — точка входа CLI-клиента GophKeeper.
package main

import (
	"os"

	"gophkeeper/internal/client/cli"
)

// Заполняются через -ldflags при сборке.
var (
	Version   = "dev"
	BuildDate = "unknown"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, cli.VersionInfo{
		Version:   Version,
		BuildDate: BuildDate,
	}))
}
