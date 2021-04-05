package main

import (
	"context"
	"flag"
	"golang.org/x/sync/errgroup"
	"log"
	"path"
	"time"

	"github.com/mariusor/feeds"
)

const (
	halfDay   = time.Hour * 12
	chunkSize = 10
)

func main() {
	var (
		basePath string
		verbose bool
	)
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.BoolVar(&verbose, "verbose", false, "Output debugging messages")
	flag.Parse()

	basePath = path.Clean(basePath)

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	all, err := feeds.GetFeeds(c)
	if err != nil {
		log.Fatal(err)
	}

	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			f := all[j]
			if f.URL == nil {
				continue
			}

			g.Go(func() error {
				if f.URL.Scheme == "" {
					log.Printf("Feed %s has an invalid URL, skipping...", f.Title)
					return nil
				}
				log.Printf("Feed %s\n", f.URL.String())
				if f.Frequency == 0 {
					f.Frequency = halfDay
				}
				var last time.Duration = 0
				if !f.Updated.IsZero() {
					last = time.Now().UTC().Sub(f.Updated)
					log.Printf("Last checked %s ago", last.Round(time.Minute).String())
				}
				if last > 0 && last <= f.Frequency {
					log.Printf(" ...newer than %s, skipping.\n", f.Frequency.String())
					return nil
				}

				if _, err = feeds.CheckFeed(f, c); err != nil {
					log.Printf("Error: %s", err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}
