package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"path"
	"sync"

	"github.com/mariusor/feeds"
	"golang.org/x/sync/errgroup"
)

const chunkSize = 20

func main() {
	var (
		basePath string
		verbose  bool
	)
	flag.StringVar(&basePath, "path", ".cache", "Base path")
	flag.BoolVar(&verbose, "verbose", false, "Output debugging messages")
	flag.Parse()

	basePath = path.Clean(basePath)
	htmlBasePath := path.Join(basePath, feeds.HtmlDir)
	if _, err := os.Stat(htmlBasePath); os.IsNotExist(err) {
		// fail if can't load html folder
		log.Fatalf("Invalid html folder %s", htmlBasePath)
		os.Exit(1)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	all, err := feeds.GetContentsForEbook(c, feeds.ValidEbookTypes[:]...)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	if len(all) == 0 {
		log.Printf("Nothing to do, exiting.")
	}

	m := sync.Mutex{}
	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			item := &all[j]
			g.Go(func() error {
				defer m.Unlock()

				m.Lock()
				if err := generateContent(item, basePath, true); err != nil {
					feeds.MarkItemsAsFailed(c, *item)
					return nil
				}

				if err = feeds.InsertContent(c, *item); err != nil {
					log.Printf("Unable to update paths in db: %s", err.Error())
					return nil
				}
				log.Printf("Updated content items [%d] %s: %v", item.ID, item.Title, item.Content)
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func generateContent(item *feeds.Item, basePath string, overwrite bool) error {
	if err := feeds.GenerateContent(feeds.OutputTypeHTML, basePath, item, overwrite); err != nil {
		log.Printf("Unable to generate path: %s", err.Error())
		if errors.Is(err, feeds.FileSizeError) {
			return err
		}
	}
	for _, typ := range feeds.ValidEbookTypes {
		if c, ok := item.Content[typ]; ok {
			if fileExists(c.Path) {
				continue
			}
			delete(item.Content, typ)
		}
		if err := feeds.GenerateContent(typ, basePath, item, overwrite); err != nil {
			log.Printf("Unable to generate path: %s", err.Error())
		}
	}
	return nil
}
