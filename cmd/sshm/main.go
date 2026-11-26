//go:generate go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest -64 -o resource_windows_amd64.syso versioninfo.json

package main

import (
	"fmt"
	"os"

	"github.com/Oktanetflow/sshm/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
