package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/neko233-com/gocode/internal/terminal"
	"os"
	"time"
)

func main() {
	output := flag.String("output", "", "Owned destination for pinned official Windows amd64 ConPTY")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	path, err := terminal.EnsureConPTY(ctx, *output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(path)
}
