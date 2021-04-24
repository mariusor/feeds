package feeds

import (
	"database/sql"
	"log"
	"net/url"
	"sort"
	"time"

	"github.com/SlyMarbo/rss"
	_ "modernc.org/sqlite"
)

type Feed struct {
	ID        int
	URL       *url.URL
	Title     string
	Author    string
	Frequency time.Duration
	Updated   time.Time
	Flags     int
}

func CheckFeed(f Feed, c *sql.DB) (bool, error) {
	doc, err := rss.Fetch(f.URL.String())
	if err != nil {
		return false, err
	}

	count := 0
	lastLoaded := time.Now().UTC()

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
	sort.Slice(all, func(i, j int) bool {
		return all[i].Published.Sub(all[j].Published) < 0
	})
	itemIns := `
INSERT INTO items (url, feed_id, guid, title, published_date, last_loaded, author, feed_index)
VALUES (?, ?, ?, ?, ?, ?, (select author from feeds where id = ? LIMIT 1), ifnull((select feed_index from items where feed_id = ? order by feed_index desc limit 1),0)+1);
`
	s, err = c.Prepare(itemIns)
	if err != nil {
		return false, err
	}
	for _, it := range all {
		_, err := s.Exec(it.URL.String(), it.Feed.ID, it.GUID, it.Title, it.Published.UTC().Format(time.RFC3339), lastLoaded.Format(time.RFC3339), it.Feed.ID, it.Feed.ID)
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

	updateFeed := "UPDATE feeds SET title = ?, last_loaded = ? WHERE id = ?"
	params := []interface{} {doc.Title, lastLoaded.Format(time.RFC3339), f.ID}
	if _, err = c.Exec(updateFeed, params...); err != nil {
		return false, err
	}


	return true, nil
}
