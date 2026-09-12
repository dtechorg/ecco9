// ecco9 is the Deep Tree Echo command-line interface.
package main

import (
	"os"

	"github.com/dtechorg/ecco9-cli/cli"
)

func main() {
	os.Exit(cli.Execute())
}
