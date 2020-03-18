package feeds

import (
	"database/sql"
	"github.com/SlyMarbo/rss"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"time"
)

func CheckFeed(url string, feedId int64, c *sql.DB) (bool, error) {
	doc, err := rss.Fetch(url)
	if err != nil {
		return false, err
	}

	count := 0

	updateTitle := "UPDATE feeds SET title = ?, last_loaded = ? WHERE id = ?"
	c.Exec(updateTitle, []interface{}{doc.Title, time.Now(), feedId})

	itemSel := "SELECT id FROM items WHERE url = ?"
	itemIns := "INSERT INTO items (url, feed_id, guid, title, author, published_date) VALUES(?, ?, ?, ?, ?, ?)"
	for _, item := range doc.Items {
		selItem := []interface{}{item.Link}
		var itemId int64

		s, err := c.Query(itemSel, selItem)
		if err == nil {
			err := s.Scan(&itemId)
			if err == nil && itemId > 0 {
				log.Printf("Skipping[%d] %s\n", itemId, item.Link)
				continue
			}
		}

		itemArgs := []interface{}{item.Link, feedId, item.ID, item.Date, item.Title, "unknown"}

		_, err = c.Exec(itemIns, itemArgs...)
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
