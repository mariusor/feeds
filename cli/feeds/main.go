package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/SlyMarbo/rss"
	"github.com/mxk/go-sqlite/sqlite3"

	"path"
)

const dbFilePath = "feeds.db"

const halfDay = float64(time.Hour * 12)

func createTables(c *sqlite3.Conn) (bool, error) {
	feeds := "CREATE TABLE feeds (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"url TEXT, " +
		"title TEXT, " +
		"frequency REAL, " +
		"last_loaded DATETIME" +
		")"
	c.Exec(feeds)

	items := "CREATE TABLE items (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"url TEXT, " +
		"feed_id INTEGER, " +
		"guid TEXT, " +
		"title TEXT, " +
		"author TEXT, " +
		"published_date DATETIME, " +
		"last_loaded DATETIME, " +
		"last_status INTEGER, " +
		"FOREIGN KEY(feed_id) REFERENCES feeds(id)" +
		")"
	c.Exec(items)

	contents := "CREATE TABLE items_contents (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"url TEXT, " +
		"item_id INTEGER, " +
		"html_path TEXT, " +
		"mobi_path TEXT, " +
		"dispatched INTEGER DEFAULT 0, " +
		"FOREIGN KEY(item_id) REFERENCES items(id) " +
		")"
	c.Exec(contents)

	parahumans := sqlite3.NamedArgs{"$feed": "https://www.parahumans.net/feed/", "$frequency": 120000000000, "$date": nil}
	insert := "INSERT INTO feeds (url, frequency, last_loaded) VALUES($feed, $frequency, $date)"
	c.Exec(insert, parahumans)

	users := "CREATE TABLE users ( id INTEGER PRIMARY KEY ASC);"
	c.Exec(users)

	outputs := "CREATE TABLE outputs (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"user_id INTEGER," +
		"type TEXT, " +
		"credentials TEXT" +
		");"
	c.Exec(outputs)

	targets := "create table targets (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"output_id INTEGER, " +
		"data TEXT, " +
		"FOREIGN KEY(user_id) REFERENCES users(id), " +
		"FOREIGN KEY(output_id) REFERENCES outputs(id)" +
		");"
	c.Exec(targets)

	return true, nil
}

func checkFeed(url string, feedId int64, c *sqlite3.Conn) (bool, error) {
	doc, err := rss.Fetch(url)
	if err != nil {
		return false, err
	}

	count := 0

	updateTitle := "UPDATE feeds SET title = $title, last_loaded = $now WHERE id = $id"
	c.Exec(updateTitle, sqlite3.NamedArgs{"$title": doc.Title, "$now": time.Now(), "$id": feedId})

	itemSel := "SELECT id FROM items WHERE url = $url"
	itemIns := "INSERT INTO items (url, feed_id, guid, title, author, published_date) VALUES($url, $feed_id, $guid, $title, $author, $published_at)"
	for _, item := range doc.Items {
		selItem := sqlite3.NamedArgs{"$url": item.Link}
		var itemId int64

		s, err := c.Query(itemSel, selItem)
		if err == nil {
			err := s.Scan(&itemId)
			if err == nil && itemId > 0 {
				log.Printf("Skipping[%d] %s\n", itemId, item.Link)
				continue
			}
		}

		itemArgs := sqlite3.NamedArgs{
			"$url":          item.Link,
			"$feed_id":      feedId,
			"$guid":         item.ID,
			"$published_at": item.Date,
			"$title":        item.Title,
			"$author":       "unknown",
		}

		err = c.Exec(itemIns, itemArgs)
		if err != nil {
			log.Printf("Error: %s", err)
			continue
		}

		count++
	}

	if count == 0 {
		log.Printf("No new articles\n")
	} else {
		log.Printf("%d new articles\n", count)
	}

	return true, nil
}

func main() {
	var c *sqlite3.Conn
	var err error

	var basePath string
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.Parse()

	basePath = path.Clean(basePath)

	dbPath := path.Join(basePath, dbFilePath)
	if _, err = os.Stat(dbPath); os.IsNotExist(err) {
		d, _ := sqlite3.Open(dbPath)
		defer d.Close()

		createTables(d)
		log.Println("Created feed db.")
	}

	c, _ = sqlite3.Open(dbPath)
	defer c.Close()

	sql := "SELECT id, url, frequency, last_loaded, title FROM feeds"
	var s *sqlite3.Stmt
	for s, err = c.Query(sql); err == nil; err = s.Next() {
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
		if !lastLoaded.IsZero() {}
		last := time.Now().Sub(lastLoaded)
		log.Printf("Last checked %s ago", last.String())
		if last.Seconds() > feedFrequency {
			log.Printf(" ...newer than %s, skipping.\n", time.Duration(feedFrequency).String())
			continue
		} else {
			log.Println(" ...loading")
		}

		_, err = checkFeed(url, feedId, c)

		if err != nil {
			log.Fatal(err)
			continue
		}
		args := sqlite3.NamedArgs{"$now": time.Now()}
		updateFeed := "UPDATE feeds SET lastLoaded = $now"
		c.Exec(updateFeed, args)
	}
}
