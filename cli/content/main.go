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

	all, err := feeds.GetNonFetchedItems(c)
	if err != nil {
		log.Printf("Error: %s", err)
	}
	for _, it := range all {
		log.Printf("Loading[%4d] %s [%s]", it.ID, it.URL.String(), "OK")
		htmlPath := path.Join(htmlBasePath, it.Feed.Title)
		if _, err = os.Stat(htmlPath); os.IsNotExist(err) {
			err = os.Mkdir(htmlPath, 0755)
		}
		err = feeds.LoadItem(it, c, htmlPath)
		if err != nil {
			log.Fatal(err)
			continue
		}
	}
}
