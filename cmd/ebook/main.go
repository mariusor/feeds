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
	"html",
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

func generateEbook(typ, basePath string, item *feeds.Item, overwrite bool) error {
	if !validEbookType(typ) {
		return fmt.Errorf("invalid ebook type %s, valid ones are %v", typ, validEbookTypes)
	}
	if c, ok := item.Content[typ]; ok && c.Path != "" {
		return nil
	}
	var (
		ebookFn func(content []byte, title string, author string, outPath string) error
		contFn func() ([]byte, error)
		err     error
	)
	needsHtmlFn := func(typ string) func() ([]byte, error) {
		return func() ([]byte, error) {
			c, ok := item.Content[typ]
			if !ok {
				return nil, fmt.Errorf("invalid content of type %s for item %v", typ, item)
			}
			buf, err := os.ReadFile(c.Path)
			if err != nil {
				return nil, err
			}
			return buf, nil
		}
	}
	switch typ {
	case "mobi":
		contFn = needsHtmlFn("html")
		ebookFn = func(content []byte, title string, author string, outPath string) error {
			if err := feeds.ToMobi(content, title, author, outPath); err != nil {
				return err
			}
			item.Content["mobi"] = feeds.Content{Path: outPath, Type: "mobi"}
			return nil
		}
	case "epub":
		contFn = needsHtmlFn("html")
		ebookFn = func(content []byte, title string, author string, outPath string) error {
			if err := feeds.ToEPub(content, title, author, outPath); err != nil {
				return err
			}
			item.Content["epub"] = feeds.Content{Path: outPath, Type: "epub"}
			return nil
		}
	case "html":
		contFn = needsHtmlFn("raw")
		ebookFn = func(content []byte, title string, author string, outPath string) error {
			if err = feeds.ToReadableHtml(content, outPath); err != nil {
				return err
			}
			item.Content["html"] = feeds.Content{Path: outPath, Type: "html"}
			return nil
		}
	}
	ebookPath := path.Join(basePath, feeds.OutputDir, strings.TrimSpace(item.Feed.Title), typ, item.Path(typ))
	if !path.IsAbs(ebookPath) {
		if ebookPath, err = filepath.Abs(ebookPath); err != nil {
			return err
		}
	}
	ebookDirPath := path.Dir(ebookPath)
	if _, err := os.Stat(ebookDirPath); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(ebookDirPath, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(ebookPath); !overwrite && !(err != nil && os.IsNotExist(err)) {
		return nil
	}
	buf, err := contFn()
	if err != nil {
		return err
	}
	if err = ebookFn(buf, strings.TrimSpace(item.Title), strings.TrimSpace(item.Author), ebookPath); err != nil {
		return err
	}
	if _, err := os.Stat(ebookPath); err != nil {
		return err
	}

	return nil
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

	all, err := feeds.GetContentsForEbook(c, validEbookTypes[:]...)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	if len(all) == 0 {
		log.Printf("Nothing to do, exiting.")
	}

	insEbookContent := "INSERT INTO contents (item_id, path, type) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;"
	s, err := c.Prepare(insEbookContent)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer s.Close()

	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			item  := &all[j]
			g.Go(func() error {
				err := generateContent(item, basePath, true)
				for typ, cont := range item.Content {
					if typ == "raw" {
						continue
					}
					if _, err = s.Exec(item.ID, cont.Path, typ); err != nil {
						log.Printf("Unable to update paths in db: %s", err.Error())
						return nil
					}
					log.Printf("Updated content item type %s [%d]: %s", typ, item.ID, item.Title)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func generateContent(item *feeds.Item, basePath string, overwrite bool) error {
	if err := generateEbook("html", basePath, item, overwrite); err != nil {
		log.Printf("Unable to generate path: %s", err.Error())
	}
	for _, typ := range validEbookTypes {
		if c, ok := item.Content[typ]; ok {
			if fileExists(c.Path) {
				continue
			}
			delete(item.Content, typ)
		}
		if err := generateEbook(typ, basePath, item, overwrite); err != nil {
			log.Printf("Unable to generate path: %s", err.Error())
		}
	}
	return nil
}