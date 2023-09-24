package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
	"github.com/ydnar/wasm-tools-go/cmd/wit-bindgen-go/cmd/describe"
	"github.com/ydnar/wasm-tools-go/cmd/wit-bindgen-go/cmd/print"
)

func main() {
	cmd := &cli.Command{
		Name:  "wit-bindgen-go",
		Usage: "inspect or manipulate WebAssembly Interface Types for Go",
		Commands: []*cli.Command{
			describe.Command,
			print.Command,
		},
	}

	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
