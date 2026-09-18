// Command vivi is a terminal browser for HashiCorp Vault.
package main

import (
	"os"

	"github.com/lucasassuncao/vivi/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
