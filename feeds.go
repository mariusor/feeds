package feeds

import (
	"database/sql"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/SlyMarbo/rss"
	_ "modernc.org/sqlite"
)

type Feed struct {
	ID         int
	URL        *url.URL
	Title      string
	Author     string
	Frequency  time.Duration
	Updated    time.Time
	LastStatus int
	Flags      int
}

const (
	TypeRSS  = "rss"
	TypeHTML = "html"
)

func sourceType(contentType string, body []byte) string {
	var typ string
	if len(contentType) > 0 {
		mimeType, _, _ := mime.ParseMediaType(contentType)
		parts := strings.Split(mimeType, "/")
		if len(parts) > 1 {
			typ = parts[1]
		}
	}
	if typ != "" {
		typ = http.DetectContentType(body)
	}
	if typ == "html" {
		return TypeHTML
	}
	return TypeRSS
}

func GetFeedInfo(u url.URL) (*rss.Feed, error) {
	client := http.DefaultClient
	resp, err := client.Get(u.String())

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return rss.Parse(body)
}

func CheckFeed(f Feed, c *sql.DB) (bool, error) {
	client := http.DefaultClient
	resp, err := client.Get(f.URL.String())

	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if sourceType(resp.Header.Get("Content-Type"), body) == TypeHTML {
		// The source needs processing from HTML to RSS
		if body, err = ToFeed(c, f.URL, body); err != nil {
			return false, err
		}
	}

	doc, err := rss.Parse(body)
	if err != nil {
		return false, err
	}

	count := 0
	lastLoaded := time.Now().UTC()

	itemSel := "SELECT id, published_date FROM items WHERE url = ?"
	s, err := c.Prepare(itemSel)
	if err != nil {
		return false, err
	}
	defer s.Close()
	all := make([]Item, 0)
	for _, item := range doc.Items {
		it := Item{}
		if err = s.QueryRow(item.Link).Scan(&it.ID, &it.Published); err != nil {
			log.Printf("Error: %s", err)
		}
		if item.Date.Sub(it.Published) <= 0 {
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
	itemUpd := ` UPDATE items SET url = ?, guid = ?, title = ?, published_date = ?, last_loaded = ? WHERE id = ?;`
	i, err := c.Prepare(itemIns)
	if err != nil {
		return false, err
	}
	u, err := c.Prepare(itemUpd)
	for _, it := range all {
		if it.ID > 0 {
			_, err = u.Exec(it.URL.String(), it.GUID, it.Title, it.Published.UTC().Format(time.RFC3339), lastLoaded.Format(time.RFC3339), it.ID)
			log.Printf("Updated: %s", it.URL)
		} else {
			_, err = i.Exec(it.URL.String(), it.Feed.ID, it.GUID, it.Title, it.Published.UTC().Format(time.RFC3339), lastLoaded.Format(time.RFC3339), it.Feed.ID, it.Feed.ID)
			log.Printf("Added: %s", it.URL)
		}
		if err != nil {
			log.Printf("Error for %s: %s", it.URL.String(), err)
		}
	}

	if count == 0 {
		log.Printf("No new articles\n")
	} else {
		log.Printf("%d new articles\n", count)
	}

	updateFeed := "UPDATE feeds SET title = ?, last_loaded = ? WHERE id = ?"
	params := []interface{}{doc.Title, lastLoaded.Format(time.RFC3339), f.ID}
	if _, err = c.Exec(updateFeed, params...); err != nil {
		return false, err
	}

	return true, nil
}
