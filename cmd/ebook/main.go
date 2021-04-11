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

func generateEbook(typ, basePath string, item feeds.Item, overwrite bool) (string, error) {
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
		strings.TrimSpace(item.Feed.Title),
		typ,
		item.Path(typ),
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

	html, ok := item.Content["html"]
	if !ok {
		return "", fmt.Errorf("invalid html for item %v", item)
	}
	var buf []byte
	if buf, err = os.ReadFile(html.Path); err != nil {
		return "", err
	}

	if _, err := os.Stat(ebookPath); !overwrite && !(err != nil && os.IsNotExist(err)) {
		return ebookPath, nil
	}
	if err = ebookFn(buf, strings.TrimSpace(item.Title), strings.TrimSpace(item.Author), ebookPath); err != nil {
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

	insEbookContent := "INSERT INTO contents (item_id, path, type) VALUES (?, ?, ?)"
	s, err := c.Prepare(insEbookContent)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer s.Close()

	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			item  := all[j]
			g.Go(func() error {
				cont, ok := item.Content["html"]
				if !ok {
					return fmt.Errorf("invalid html path for item %v", item)
				}
				log.Printf("File %s\n", path.Base(cont.Path))
				for _, typ := range validEbookTypes {
					filePath, err := generateEbook(typ, basePath, item, true)
					if err != nil {
						log.Printf("Unable to generate path %s: %s", filePath, err.Error())
					}
					cont.Path = filePath
					if _, err = s.Exec(cont.ID, cont.Path, typ); err != nil {
						log.Printf("Unable to update paths in db: %s", err.Error())
						return nil
					}
					log.Printf("Updated content item [%d]: %s", cont.ID, item.Title)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}
