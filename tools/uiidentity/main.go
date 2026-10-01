package main

import (
	"encoding/json"
	"fmt"
	"os"

	adminui "github.com/chennqqi/rabbit-jetstream/admin-ui"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "uiidentity accepts no arguments")
		os.Exit(2)
	}
	if err := json.NewEncoder(os.Stdout).Encode(adminui.EmbeddedAssetIdentity()); err != nil {
		fmt.Fprintln(os.Stderr, "encode embedded UI identity")
		os.Exit(1)
	}
}
