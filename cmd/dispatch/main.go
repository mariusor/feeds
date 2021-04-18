package main

import (
	"database/sql"
	"flag"
	"fmt"
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
		if err := Dispatch(c, disp); err != nil {
			log.Printf("Error dispatching: %s", err)
		}
	}
}

func Dispatch(c *sql.DB, disp feeds.DispatchItem) error {
	var (
		err error
		status bool
	)
	switch disp.Destination.Type {
	case "myk":
		status, err = feeds.DispatchToKindle(disp)
	case "pocket":
		status, err = feeds.DispatchToPocket(disp)
	default:
		return fmt.Errorf("unknown dispatch type %s", disp.Destination.Type)
	}
	disp.LastStatus = status
	if err != nil {
		disp.LastMessage = err.Error()
	}
	feeds.SaveTarget(c, disp)
	return err
}
