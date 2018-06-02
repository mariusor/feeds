package main

import (
	"github.com/mxk/go-sqlite/sqlite3"
	"github.com/766b/mobi"
	"flag"
	"path"
	"os"
	"log"
	"bytes"
	"path/filepath"
)

const htmlDir = "articles"
const mobiDir = "output/mobi"
const dbFilePath = "feeds.db"

func toMobi(content []byte, title string, author string, outPath string) error {
	m, err := mobi.NewWriter(outPath)
	if err != nil {
		return err
	}

	m.Title(title)
	m.Compression(mobi.CompressionPalmDoc)
	m.NewExthRecord(mobi.EXTH_DOCTYPE, "PBOK")
	if author != "" {
		m.NewExthRecord(mobi.EXTH_AUTHOR, author)
	}
	m.NewChapter(title, content)
	// Output MOBI File
	m.Write()
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
		// fail if can't load html folder
		log.Fatalf("Invalid html folder %s", htmlBasePath)
		os.Exit(1)
	}
	mobiBasePath := path.Dir(path.Join(basePath, mobiDir))
	if _, err = os.Stat(mobiBasePath); os.IsNotExist(err) {
		os.Mkdir(mobiBasePath, 0755)
	}

	c, _ = sqlite3.Open(dbPath)
	defer c.Close()

	sql := "SELECT items_contents.id, feeds.title, items.title, items.author, html_path " +
	"FROM items_contents " +
	"INNER JOIN items ON items.id = items_contents.item_id " +
	"INNER JOIN feeds ON feeds.id = items.feed_id WHERE mobi_path IS NULL"
	var s *sqlite3.Stmt

	for s, err = c.Query(sql); err == nil; err = s.Next() {
		var itemId int64
		var feedTitle string
		var itemTitle string
		var itemAuthor string
		var htmlPath string

		s.Scan(&itemId, &feedTitle, &itemTitle, &itemAuthor, &htmlPath)
		log.Printf("File %s\n", path.Base(htmlPath))

		f, err := os.Open(htmlPath)
		if err != nil{
			log.Fatal(err)
			os.Exit(1)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(f)
		f.Close()

		mobiPath := path.Join(mobiBasePath, feedTitle)
		if _, err = os.Stat(mobiPath); os.IsNotExist(err) {
			err = os.Mkdir(mobiPath, 0755)
		}
		mobiPath = path.Join(mobiPath, itemTitle + ".mobi")
		if !path.IsAbs(mobiPath) {
			mobiPath, _ = filepath.Abs(mobiPath)
		}
		err = toMobi(buf.Bytes(), itemTitle, itemAuthor, mobiPath)
		if err != nil {
			log.Fatalf("Unable to save file %s", mobiPath)
			continue
		}
		args := sqlite3.NamedArgs{"$mobi_path": mobiPath, "$id": itemId}
		updateFeed := "UPDATE items_contents SET mobi_path = $mobi_path WHERE id = $id"
		c.Exec(updateFeed, args)
	}

	if err != nil {
		log.Fatal(err)
	}
}