package main

import (
	"flag"
	"log"
	"time"

	"github.com/mariusor/feeds"
	_ "github.com/mattn/go-sqlite3"

	"path"
)

const halfDay = time.Hour * 1

func main() {
	var basePath string
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
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

	for _, f := range all {
		log.Printf("Feed %s\n", f.URL.String())
		if f.Frequency == 0.0 {
			f.Frequency = halfDay
		}
		var last time.Duration = 0
		if !f.Updated.IsZero() {
			last = time.Now().Sub(f.Updated)
			log.Printf("Last checked %s ago", last.String())
		}
		if last > f.Frequency {
			log.Printf(" ...newer than %s, skipping.\n", time.Duration(f.Frequency).String())
			continue
		}

		_, err = feeds.CheckFeed(f, c)
		if err != nil {
			log.Fatal(err)
			continue
		}
		updateFeed := "UPDATE feeds SET lastLoaded = ?"
		c.Exec(updateFeed, time.Now())
	}
}
