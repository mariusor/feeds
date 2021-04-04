package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mariusor/feeds"
	"golang.org/x/sync/errgroup"
)

const chunkSize = 20

var validEbookTypes = [...]string {
	"epub",
	"mobi",
}

func validEbookType(typ string) bool {
	for _, t := range validEbookTypes {
		if t == typ {
			return true
		}
	}
	return false
}

func generateEbook(typ, basePath string, cont feeds.Content, overwrite bool) (string, error) {
	if !validEbookType(typ) {
		return "", fmt.Errorf("invalid ebook type %s, valid ones are %v", typ, validEbookTypes)
	}
	var (
		ebookFn func(content []byte, title string, author string, outPath string) error
		err error
	)
	switch typ {
	case "mobi":
		ebookFn = feeds.ToMobi
	case "epub":
		ebookFn = feeds.ToEPub
	}
	ebookPath := path.Join(
		basePath,
		feeds.OutputDir,
		strings.TrimSpace(cont.Item.Feed.Title),
		typ,
		cont.Item.Path(typ),
	)
	if !path.IsAbs(ebookPath) {
		if ebookPath, err = filepath.Abs(ebookPath); err != nil {
			return "", err
		}
	}
	ebookDirPath := path.Dir(ebookPath)
	if _, err := os.Stat(ebookDirPath); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(ebookDirPath, 0755); err != nil {
			return "", err
		}
	}
	var buf []byte
	if buf, err = os.ReadFile(cont.HTMLPath); err != nil {
		return "", err
	}

	if _, err := os.Stat(ebookPath); !overwrite && !(err != nil && os.IsNotExist(err)) {
		return ebookPath, nil
	}
	if err = ebookFn(buf, strings.TrimSpace(cont.Item.Title), strings.TrimSpace(cont.Item.Author), ebookPath); err != nil {
		return "", err
	}
	if _, err := os.Stat(ebookPath); err != nil {
		return "", err
	}

	return ebookPath, nil
}

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
		os.Exit(1)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	all, err := feeds.GetContentsForEbook(c)
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
				for _, typ := range validEbookTypes {
					filePath, err := generateEbook(typ, basePath, cont, true)
					if err != nil {
						log.Printf("Unable to generate path %s: %s", filePath, err.Error())
					}
					switch typ {
					case "mobi":
						cont.MobiPath = filePath
					case "epub":
						cont.EPubPath = filePath
					}
				}
				if _, err = s.Exec(cont.MobiPath, cont.EPubPath, cont.ID); err != nil {
					log.Printf("Unable to update paths in db: %s", err.Error())
					return nil
				}
				log.Printf("Updated content items [%d]: %s", cont.ID, cont.Item.Title)

				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}
