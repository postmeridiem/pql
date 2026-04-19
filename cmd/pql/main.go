// Command pql is the project query language CLI. See docs/structure/ for design.
package main

import (
	"os"

	"github.com/postmeridiem/pql/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
