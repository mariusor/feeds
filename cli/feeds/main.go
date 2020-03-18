package main

import (
	"flag"
	"log"
	"time"

	"github.com/mariusor/feeds"
	_ "github.com/mattn/go-sqlite3"

	"path"
)

const halfDay = float64(time.Hour * 12)

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

	sql := "SELECT id, url, frequency, last_loaded, title FROM feeds"
	s, err := c.Query(sql)
	if err != nil {
		log.Fatal(err)
	}
	for s.Next() {
		var feedId int64
		var feedFrequency float64
		var feedTitle string
		var url string
		var lastLoaded time.Time

		s.Scan(&feedId, &url, &feedFrequency, &lastLoaded, &feedTitle)
		log.Printf("Feed %s\n", url)
		if feedFrequency == 0.0 {
			feedFrequency = halfDay
		}
		if !lastLoaded.IsZero() {
		}
		last := time.Now().Sub(lastLoaded)
		log.Printf("Last checked %s ago", last.String())
		if last.Seconds() > feedFrequency {
			log.Printf(" ...newer than %s, skipping.\n", time.Duration(feedFrequency).String())
			continue
		} else {
			log.Println(" ...loading")
		}

		_, err = feeds.CheckFeed(url, feedId, c)

		if err != nil {
			log.Fatal(err)
			continue
		}
		args := []interface{}{time.Now()}
		updateFeed := "UPDATE feeds SET lastLoaded = ?"
		c.Exec(updateFeed, args)
	}
}
