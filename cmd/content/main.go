package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/mariusor/feeds"
	"github.com/mariusor/go-readability"

	"golang.org/x/sync/errgroup"
)

const (
	chunkSize = 5
	defaultSleepAfterBatch = 200*time.Millisecond
)

func main() {
	var (
		basePath string
		verbose bool
	)
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.BoolVar(&verbose, "verbose", false, "Output debugging messages")
	flag.Parse()

	if verbose {
		readability.Logger = log.New(os.Stdout, "[readability] ", log.LstdFlags)
	}

	basePath = path.Clean(basePath)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, 0755)
	}

	htmlBasePath := path.Join(basePath, feeds.HtmlDir)
	if _, err := os.Stat(htmlBasePath); os.IsNotExist(err) {
		os.Mkdir(htmlBasePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	all, err := feeds.GetNonFetchedItems(c)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			it := all[j]
			htmlPath := path.Join(htmlBasePath, strings.TrimSpace(it.Feed.Title))
			if _, err = os.Stat(htmlPath); os.IsNotExist(err) {
				err = os.Mkdir(htmlPath, 0755)
			}
			g.Go(func() error {
				err := feeds.LoadItem(it, c, htmlPath)
				log.Printf("Loaded[%5d] %s [%s]", it.FeedIndex, it.URL.String(), "OK")
				return err
			})
			time.Sleep(defaultSleepAfterBatch)
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}
