package main

import (
	"bytes"
	"flag"
	"github.com/mariusor/feeds"
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

	all, err := feeds.GetContentsForMobi(c)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	updateFeed := "UPDATE items_contents SET mobi_path = ? WHERE id = ?"
	s, err := c.Prepare(updateFeed)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer s.Close()
	for _, cont := range all {
		log.Printf("File %s\n", path.Base(cont.HTMLPath))

		f, err := os.Open(cont.HTMLPath)
		if err != nil {
			log.Fatal("Error: %s", err)
		}
		defer f.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(f)

		mobiPath := path.Join(mobiBasePath, cont.Item.Feed.Title)
		if _, err = os.Stat(mobiPath); os.IsNotExist(err) {
			err = os.Mkdir(mobiPath, 0755)
		}
		mobiPath = path.Join(mobiPath, cont.Item.Title+".mobi")
		if !path.IsAbs(mobiPath) {
			mobiPath, _ = filepath.Abs(mobiPath)
		}

		if _, err := os.Stat(mobiPath); os.IsNotExist(err) {
			err = feeds.ToMobi(buf.Bytes(), cont.Item.Title, cont.Item.Author, mobiPath)
			if err != nil {
				log.Printf("Unable to save file %s", mobiPath)
				continue
			}
		}
		if _, err := os.Stat(mobiPath); err != nil {
			log.Printf("Error: %s", err)
			continue
		}
		_, err = s.Exec(mobiPath, cont.ID)
		if err != nil {
			log.Printf("Unable to update path %s", mobiPath)
		} else {
			log.Printf("Updated content items [%d]: %s", cont.ID, mobiPath)
		}
	}

}
