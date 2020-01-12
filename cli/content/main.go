package main

import (
	"flag"
	"log"
	"os"

	"github.com/mariusor/go-readability"
	"github.com/mxk/go-sqlite/sqlite3"

	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
)

const htmlDir = "articles"
const dbFilePath = "feeds.db"

func toReadableHtml(content []byte) ([]byte, string, error) {
	var err error
	var doc *readability.Document

	doc, err = readability.NewDocument(string(content))
	if err != nil {
		return []byte{}, "err::title", err
	}

	return []byte(doc.Content()), doc.Title, nil
}

func loadItem(id int64, url string, c *sqlite3.Conn, htmlPath string) error {
	contentIns := "INSERT INTO items_contents (url, item_id, html_path) VALUES($url, $item_id, $html_path)"

	res, err := http.Get(url)
	if err != nil {
		return err
	}
	log.Printf("Loading %s [%s]", url, "OK")
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	res.Body.Close()

	content, title, err := toReadableHtml(data)
	if err != nil {
		return err
	}
	// write html to path
	outPath := path.Join(htmlPath, title+".html")
	err = ioutil.WriteFile(outPath, content, 0644)
	if err != nil {
		return err
	}
	if !path.IsAbs(outPath) {
		outPath, _ = filepath.Abs(outPath)
	}
	itemArgs := sqlite3.NamedArgs{
		"$url":       url,
		"$item_id":   id,
		"$html_path": outPath,
	}

	err = c.Exec(contentIns, itemArgs)
	if err != nil {
		return err
	}

	return nil
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
		// fail if can't load db
		log.Fatalf("Could not open db %s", dbPath)
		os.Exit(1)
	}

	htmlBasePath := path.Join(basePath, htmlDir)
	if _, err = os.Stat(htmlBasePath); os.IsNotExist(err) {
		os.Mkdir(htmlBasePath, 0755)
	}

	c, _ = sqlite3.Open(dbPath)
	defer c.Close()

	sql := "SELECT items.id, feeds.title, items.url FROM items INNER JOIN feeds LEFT JOIN items_contents ON items_contents.item_id = items.id WHERE items_contents.id IS NULL"
	var s *sqlite3.Stmt
	for s, err = c.Query(sql); err == nil; err = s.Next() {
		var itemId int64
		var feedTitle string
		var itemUrl string

		s.Scan(&itemId, &feedTitle, &itemUrl)

		htmlPath := path.Join(htmlBasePath, feedTitle)
		if _, err = os.Stat(htmlPath); os.IsNotExist(err) {
			err = os.Mkdir(htmlPath, 0755)
		}
		err = loadItem(itemId, itemUrl, c, htmlPath)
		if err != nil {
			log.Fatal(err)
			continue
		}
	}
}
