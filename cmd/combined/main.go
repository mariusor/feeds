package main

import (
	"context"
	"github.com/mariusor/go-readability"
	"log"
	"os"
	"path"

	"git.sr.ht/~mariusor/ssm"
	"github.com/alecthomas/kong"
	"github.com/mariusor/feeds/internal"
)

var CLI struct {
	Path    string `default:".cache" help:"Base storage path"`
	Verbose bool   `short:"v" help:"Output debugging messages"`
}

var logFlags = log.Lshortfile | log.LstdFlags | log.LUTC | log.Lmsgprefix

func main() {
	log.SetFlags(logFlags)
	kong.Parse(&CLI,
		kong.Name("combined"),
		kong.Description("State machine based executor for all steps of the feed loader"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}))

	if CLI.Verbose {
		readability.Logger = log.New(os.Stderr, "[rdb] ", logFlags)
	}

	basePath := path.Clean(CLI.Path)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, 0755)
	}

	ctx := context.Background()

	sm := internal.SM{Path: basePath}

	if err := ssm.Run(ctx, sm.Wait); err != nil {
		log.Fatal(err.Error())
	}
}
