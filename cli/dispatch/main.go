package main

import (
	"flag"
	"github.com/mariusor/feeds"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"path"
)

func main() {
	var basePath string
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.Parse()

	basePath = path.Clean(basePath)
	htmlBasePath := path.Join(basePath, feeds.HtmlDir)
	if _, err := os.Stat(htmlBasePath); os.IsNotExist(err) {
		// fail if can't load html folder
		log.Fatalf("Invalid html folder %s", htmlBasePath)
		os.Exit(1)
	}
	mobiBasePath := path.Dir(path.Join(basePath, feeds.MobiDir))
	if _, err := os.Stat(mobiBasePath); os.IsNotExist(err) {
		os.Mkdir(mobiBasePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	sql := "SELECT items_contents.id, feeds.title, items.title, author, mobi_path " +
		"FROM items_contents " +
		"INNER JOIN items ON items.id = items_contents.item_id " +
		"INNER JOIN feeds ON feeds.id = items.feed_id WHERE items_contents.dispatched != 1"

	s, err := c.Query(sql)
	defer s.Close()
	if err != nil {
		log.Fatal(err)
	}
	for s.Next() {
		var itemId int64
		var feedTitle string
		var itemTitle string
		var itemAuthor string
		var mobiPath string

		s.Scan(&itemId, &feedTitle, &itemTitle, &itemAuthor, &mobiPath)

		err = feeds.DispatchToKindle(itemTitle, mobiPath, c)
		if err != nil {
			log.Fatal(err)
			continue
		}
		args := []interface{}{true, itemId}
		updateFeed := "UPDATE items_contents SET dispatched = ? WHERE id = ?"
		c.Exec(updateFeed, args...)
	}
}
