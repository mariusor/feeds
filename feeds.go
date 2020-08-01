package feeds

import (
	"database/sql"
	"github.com/SlyMarbo/rss"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/url"
	"time"
)

type Feed struct {
	ID        int
	URL       *url.URL
	Title     string
	Author    string
	Frequency time.Duration
	Updated   time.Time
}

func CheckFeed(f Feed, c *sql.DB) (bool, error) {
	doc, err := rss.Fetch(f.URL.String())
	if err != nil {
		return false, err
	}

	count := 0

	updateTitle := "UPDATE feeds SET title = ?, last_loaded = ? WHERE id = ?"
	c.Exec(updateTitle, doc.Title, time.Now().UTC(), f.ID)

	itemSel := "SELECT id FROM items WHERE url = ?"
	s, err := c.Prepare(itemSel)
	if err != nil {
		return false, err
	}
	defer s.Close()
	all := make([]Item, 0)
	for _, item := range doc.Items {
		it := Item{}
		err = s.QueryRow(item.Link).Scan(&it.ID)
		if err == nil && it.ID > 0 {
			log.Printf("Skipping[%d] %s\n", it.ID, item.Link)
			continue
		}
		it.Feed.ID = f.ID
		it.GUID = item.ID
		it.Published = item.Date
		it.Title = item.Title
		it.Author = f.Author
		it.URL, _ = url.Parse(item.Link)
		all = append(all, it)

		count++
	}
	itemIns := "INSERT INTO items (url, feed_id, guid, title, author, published_date) VALUES(?, ?, ?, ?, ?, ?)"
	s, err = c.Prepare(itemIns)
	if err != nil {
		return false, err
	}
	for _, it := range all {
		_, err := s.Exec(it.URL.String(), it.Feed.ID, it.GUID, it.Title, it.Author, it.Published)
		if err != nil {
			log.Printf("Error: %s", err)
			continue
		}
	}

	if count == 0 {
		log.Printf("No new articles\n")
	} else {
		log.Printf("%d new articles\n", count)
	}

	return true, nil
}
