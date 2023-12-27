package main

import (
	"context"
	"log"
	"os"
	"path"

	"github.com/alecthomas/kong"
	"github.com/mariusor/feeds"
)

var CLI struct {
	Path    string `default:".cache" help:"Base storage path"`
	Verbose bool   `short:"v" help:"Output debugging messages"`
}

func main() {
	kong.Parse(&CLI,
		kong.Name("content"),
		kong.Description("Command to dispatch all pending articles"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}))

	basePath := path.Clean(CLI.Path)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatalf("Failed to open database: %s", err)
	}
	defer c.Close()

	if err := feeds.DispatchContentCmd(context.Background(), c); err != nil {
		log.Fatalf("Failed to fetch items: %s", err)
		os.Exit(1)
	}
}
