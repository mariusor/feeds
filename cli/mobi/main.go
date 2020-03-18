package main

import (
	"bytes"
	"flag"
	"github.com/mariusor/feeds"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"path"
	"path/filepath"
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

	sql := "SELECT items_contents.id, feeds.title, items.title, items.author, html_path " +
		"FROM items_contents " +
		"INNER JOIN items ON items.id = items_contents.item_id " +
		"INNER JOIN feeds ON feeds.id = items.feed_id WHERE mobi_path IS NULL"

	s, err := c.Query(sql)
	if err != nil {
		log.Fatal(err)
	}
	for s.Next() {
		var itemId int64
		var feedTitle string
		var itemTitle string
		var itemAuthor string
		var htmlPath string

		s.Scan(&itemId, &feedTitle, &itemTitle, &itemAuthor, &htmlPath)
		log.Printf("File %s\n", path.Base(htmlPath))

		f, err := os.Open(htmlPath)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(f)
		f.Close()

		mobiPath := path.Join(mobiBasePath, feedTitle)
		if _, err = os.Stat(mobiPath); os.IsNotExist(err) {
			err = os.Mkdir(mobiPath, 0755)
		}
		mobiPath = path.Join(mobiPath, itemTitle+".mobi")
		if !path.IsAbs(mobiPath) {
			mobiPath, _ = filepath.Abs(mobiPath)
		}
		err = feeds.ToMobi(buf.Bytes(), itemTitle, itemAuthor, mobiPath)
		if err != nil {
			log.Fatalf("Unable to save file %s", mobiPath)
			continue
		}
		args := []interface{}{mobiPath, itemId}
		updateFeed := "UPDATE items_contents SET mobi_path = ? WHERE id = ?"
		c.Exec(updateFeed, args...)
	}
}
