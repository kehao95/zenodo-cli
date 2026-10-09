package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/kehao95/zenodo-cli/cmd"
)

var version = "dev"
var commit = "unknown"
var date = "unknown"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(cmd.Execute(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version+" (commit: "+commit+", built: "+date+")"))
}
