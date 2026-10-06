package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/neko233-com/gocode/internal/update"
)

func main() {
	root := flag.String("uninstall-cleanup", "", "Owned installation to clean during direct MSI uninstall")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "maintenance requires an explicit owned installation")
		os.Exit(1)
	}
	key, err := update.PublisherKey()
	if err == nil {
		err = update.Cleanup(*root, key)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
