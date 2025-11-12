package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/doeshing/shai-go/internal/infrastructure/cli"
)

func main() {
	ctx := context.Background()
	opts := cli.Options{Verbose: isVerbose()}

	root, err := cli.NewRootCmd(ctx, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func isVerbose() bool {
	return strings.EqualFold(os.Getenv("SHAI_DEBUG"), "1") || strings.EqualFold(os.Getenv("SHAI_DEBUG"), "true")
}
