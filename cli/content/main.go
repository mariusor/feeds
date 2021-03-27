package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path"

	"github.com/mariusor/feeds"
	"golang.org/x/sync/errgroup"
)

const chunkSize = 10

func main() {
	var basePath string
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.Parse()

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
			htmlPath := path.Join(htmlBasePath, it.Feed.Title)
			if _, err = os.Stat(htmlPath); os.IsNotExist(err) {
				err = os.Mkdir(htmlPath, 0755)
			}
			g.Go(func() error {
				err := feeds.LoadItem(it, c, htmlPath)
				log.Printf("Loaded[%5d] %s [%s]", it.ID, it.URL.String(), "OK")
				return err
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}
