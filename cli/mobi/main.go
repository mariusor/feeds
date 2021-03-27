package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mariusor/feeds"
	"golang.org/x/sync/errgroup"
)

const chunkSize = 20

func generateEPub(basePath string, cont feeds.Content) (string, error) {
	epubBasePath := path.Dir(path.Join(basePath, feeds.EPubDir))
	if _, err := os.Stat(epubBasePath); os.IsNotExist(err) {
		os.Mkdir(epubBasePath, 0755)
	}

	feedPath := path.Join(epubBasePath, cont.Item.Feed.Title)
	if _, err := os.Stat(feedPath); os.IsNotExist(err) {
		if err = os.Mkdir(feedPath, 0755); err != nil {
			return "", err
		}
	}
	buf, err := cont.HTML()
	if err != nil {
		return "", err
	}

	epubPath := path.Join(feedPath, cont.Item.Path("epub"))
	if !path.IsAbs(epubPath) {
		epubPath, _ = filepath.Abs(epubPath)
	}

	if _, err := os.Stat(epubPath); os.IsNotExist(err) {
		if err = feeds.ToEPub(buf, strings.TrimSpace(cont.Item.Title), strings.TrimSpace(cont.Item.Author), epubPath); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(epubPath); err != nil {
		return "", err
	}
	return epubPath, nil
}

func generateMobi(basePath string, cont feeds.Content) (string, error) {
	mobiBasePath := path.Dir(path.Join(basePath, feeds.MobiDir))
	if _, err := os.Stat(mobiBasePath); os.IsNotExist(err) {
		os.Mkdir(mobiBasePath, 0755)
	}

	feedPath := path.Join(mobiBasePath, cont.Item.Feed.Title)
	if _, err := os.Stat(feedPath); os.IsNotExist(err) {
		if err = os.Mkdir(feedPath, 0755); err != nil {
			return "", err
		}
	}
	buf, err := cont.HTML()
	if err != nil {
		return "", err
	}

	mobiPath := path.Join(feedPath, cont.Item.Path("mobi"))
	if !path.IsAbs(mobiPath) {
		mobiPath, _ = filepath.Abs(mobiPath)
	}

	if _, err := os.Stat(mobiPath); os.IsNotExist(err) {
		if err = feeds.ToMobi(buf, strings.TrimSpace(cont.Item.Title), strings.TrimSpace(cont.Item.Author), mobiPath); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(mobiPath); err != nil {
		return "", err
	}
	return mobiPath, nil
}

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

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	all, err := feeds.GetContentsForMobi(c)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	updateFeed := "UPDATE items_contents SET mobi_path = ?, epub_path = ? WHERE id = ?"
	s, err := c.Prepare(updateFeed)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer s.Close()

	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			cont := all[j]
			g.Go(func() error {
				log.Printf("File %s\n", path.Base(cont.HTMLPath))

				mobiPath, err := generateMobi(basePath, cont)
				if err != nil {
					log.Printf("Unable to generate path %s: %s", mobiPath, err.Error())
				}
				epubPath, err := generateEPub(basePath, cont)
				if err != nil {
					log.Printf("Unable to generate path %s: %s", epubPath, err.Error())
				}

				if len(epubPath)+len(mobiPath) > 0 {
					if _, err = s.Exec(mobiPath, epubPath, cont.ID); err != nil {
						log.Printf("Unable to update paths in db: %s", err.Error())
						return nil
					}
					log.Printf("Updated content items [%d]: %s", cont.ID, cont.Item.Title)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}
