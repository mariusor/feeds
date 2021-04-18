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

	all, err := feeds.GetNonDispatchedItemContentsForDestination(c)
	if err != nil {
		log.Printf("Error: %s", err)
	}
	for _, disp := range all {
		it := disp.Item
		for typ, cont := range it.Content {
			var st bool
			switch typ {
			case "kindle":
				if st, err = feeds.DispatchToKindle(it.Title, cont.Path, c); err != nil {
					log.Printf("Error: %s", err)
					continue
				}
			case "pocket":
				log.Printf("Pocket dispatch is not ready yet for %s", it.Title)
			}
			targetSql := `insert into targets (destination_id, item_id, last_status) VALUES(?, ?, ?);`
			c.Exec(targetSql, disp.Destination.ID, it.ID, st)
		}
	}
}
