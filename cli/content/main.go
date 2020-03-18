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
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, 0755)
	}

	htmlBasePath := path.Join(basePath, feeds.HtmlDir)
	if _, err := os.Stat(htmlBasePath); os.IsNotExist(err) {
		os.Mkdir(htmlBasePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	sql := "SELECT items.id, feeds.title, items.url FROM items INNER JOIN feeds LEFT JOIN items_contents ON items_contents.item_id = items.id WHERE items_contents.id IS NULL"
	s, err := c.Query(sql)
	defer s.Close()
	if err != nil {
		log.Fatal(err)
	}
	for s.Next() {
		var itemId int64
		var feedTitle string
		var itemUrl string

		s.Scan(&itemId, &feedTitle, &itemUrl)

		htmlPath := path.Join(htmlBasePath, feedTitle)
		if _, err = os.Stat(htmlPath); os.IsNotExist(err) {
			err = os.Mkdir(htmlPath, 0755)
		}
		err = feeds.LoadItem(itemId, itemUrl, c, htmlPath)
		if err != nil {
			log.Fatal(err)
			continue
		}
	}
}
