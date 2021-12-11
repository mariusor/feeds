package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/mariusor/feeds"
	"github.com/mariusor/go-readability"

	"golang.org/x/sync/errgroup"
)

const (
	chunkSize              = 5
	defaultSleepAfterBatch = 200 * time.Millisecond
)

func main() {
	var (
		basePath string
		verbose  bool
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

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	all, err := feeds.GetNonFetchedItems(c)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	if len(all) == 0 {
		log.Printf("Nothing to do, exiting.")
	}
	maxFailureCount := 3
	failures := make(map[int]int)
	m := sync.Mutex{}
	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			it := all[j]
			if failures[it.Feed.ID] > maxFailureCount {
				log.Printf("Skipping %s, too many failures when loading", it.URL)
				continue
			}
			g.Go(func() error {
				defer func() {
					m.Unlock()
					time.Sleep(defaultSleepAfterBatch)
				}()

				m.Lock()
				status, err := feeds.LoadItem(&it, c, basePath)
				if err != nil {
					log.Printf("Error[%5d] %s %s", it.FeedIndex, it.URL.String(), err.Error())
					failures[it.Feed.ID]++
				}
				log.Printf("Loaded[%5d] %s [%t]", it.FeedIndex, it.URL.String(), status)

				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}
