package main

import (
	"encoding/json"
	"flag"
	"github.com/jordan-wright/email"
	"github.com/mxk/go-sqlite/sqlite3"
	"log"
	"net/smtp"
	"os"
	"path"
)

const htmlDir = "articles"
const mobiDir = "output/mobi"
const dbFilePath = "feeds.db"

type smtpCreds struct {
	Server   string `json:"server"`
	Port     string `json:"port"`
	From     string `json:"from"`
	User     string `json:"user"`
	Password string `json:"password"`
}
type destination struct {
	To string `json:"to"`
}

func dispatchToKindle(subject string, attachment string, c *sqlite3.Conn) error {
	targets := "SELECT targets.data, outputs.credentials FROM targets INNER JOIN outputs ON outputs.id = targets.output_id WHERE outputs.type = 'kindle'"

	var err error
	var s *sqlite3.Stmt
	for s, err = c.Query(targets); err == nil; err = s.Next() {
		var data []byte
		var credentials []byte

		s.Scan(&data, &credentials)

		var settings smtpCreds
		var target destination
		err = json.Unmarshal(credentials, &settings)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, &target)
		if err != nil {
			return err
		}
		e := email.NewEmail()
		log.Printf("Emailing %s to %s", attachment, target.To)

		e.From = settings.From
		e.To = []string{target.To}
		e.Cc = []string{"marius.orcsik@gmail.com"}
		e.Subject = subject
		e.AttachFile(attachment)
		err = e.Send(settings.Server+":"+settings.Port, smtp.PlainAuth("", settings.User, settings.Password, settings.Server))
		if err != nil {
			return err
		}
	}
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

	sql := "SELECT items_contents.id, feeds.title, items.title, author, mobi_path " +
		"FROM items_contents " +
		"INNER JOIN items ON items.id = items_contents.item_id " +
		"INNER JOIN feeds ON feeds.id = items.feed_id WHERE items_contents.dispatched != 1"
	var s *sqlite3.Stmt

	for s, err = c.Query(sql); err == nil; err = s.Next() {
		var itemId int64
		var feedTitle string
		var itemTitle string
		var itemAuthor string
		var mobiPath string

		s.Scan(&itemId, &feedTitle, &itemTitle, &itemAuthor, &mobiPath)

		err = dispatchToKindle(itemTitle, mobiPath, c)
		if err != nil {
			log.Fatal(err)
			continue
		}
		args := sqlite3.NamedArgs{"$dispatched": true, "$id": itemId}
		updateFeed := "UPDATE items_contents SET dispatched = $dispatched WHERE id = $id"
		c.Exec(updateFeed, args)
	}
}
