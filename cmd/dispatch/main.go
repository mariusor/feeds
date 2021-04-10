package main

import (
	"flag"
	"log"
	"os"
	"path"

	"github.com/mariusor/feeds"
)

func main() {
	var (
		basePath string
		verbose bool
	)
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.BoolVar(&verbose, "verbose", false, "Output debugging messages")
	flag.Parse()

	basePath = path.Clean(basePath)
	htmlBasePath := path.Join(basePath, feeds.HtmlDir)
	if _, err := os.Stat(htmlBasePath); os.IsNotExist(err) {
		// fail if can't load html folder
		log.Fatalf("Invalid html folder %s", htmlBasePath)
	}
	mobiBasePath := path.Dir(path.Join(basePath, feeds.MobiDir))
	if _, err := os.Stat(mobiBasePath); os.IsNotExist(err) {
		os.Mkdir(mobiBasePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer c.Close()

	all, err := feeds.GetNonDispatchedItemContents(c)
	if err != nil {
		log.Printf("Error: %s", err)
	}
	for _, cont := range all {
		st, err := feeds.DispatchToKindle(cont.Item.Title, cont.Path, c)
		if err != nil {
			log.Printf("Error: %s", err)
			continue
		}
		updateFeed := "UPDATE items_contents SET dispatched = ? WHERE id = ?"
		c.Exec(updateFeed, st, cont.Item.ID)
	}
}
