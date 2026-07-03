package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rioriost/Xanship/internal/xanship"
)

func main() {
	if err := xanship.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "xanship:", err)
		os.Exit(1)
	}
}
