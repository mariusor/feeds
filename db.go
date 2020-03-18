package feeds

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const DBFilePath = "feeds.db"

func DB(basePath string) (*sql.DB, error) {
	dbPath := path.Join(basePath, DBFilePath)
	bootstrap := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		bootstrap = true
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return db, err
	}
	if bootstrap {
		if err := createTables(db); err != nil {
			return nil, err
		}
	}

	return db, nil
}

func createTables(c *sql.DB) error {
	feeds := "CREATE TABLE feeds (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"url TEXT, " +
		"title TEXT, " +
		"frequency REAL, " +
		"last_loaded DATETIME" +
		")"
	if _, err := c.Exec(feeds); err != nil {
		return err
	}

	items := "CREATE TABLE items (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"url TEXT, " +
		"feed_id INTEGER, " +
		"guid TEXT, " +
		"title TEXT, " +
		"author TEXT, " +
		"published_date DATETIME, " +
		"last_loaded DATETIME, " +
		"last_status INTEGER, " +
		"FOREIGN KEY(feed_id) REFERENCES feeds(id)" +
		")"
	if _, err := c.Exec(items); err != nil {
		return err
	}

	contents := "CREATE TABLE items_contents (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"url TEXT, " +
		"item_id INTEGER, " +
		"html_path TEXT, " +
		"mobi_path TEXT, " +
		"dispatched INTEGER DEFAULT 0, " +
		"FOREIGN KEY(item_id) REFERENCES items(id) " +
		")"
	if _, err := c.Exec(contents); err != nil {
		return err
	}

	parahumans := []interface{}{"https://www.parahumans.net/feed/", 120000000000, nil}
	insert := "INSERT INTO feeds (url, frequency, last_loaded) VALUES(?, ?, ?)"
	if _, err := c.Exec(insert, parahumans...); err != nil {
		return err
	}

	users := "CREATE TABLE users ( id INTEGER PRIMARY KEY ASC);"
	if _, err := c.Exec(users); err != nil {
		return err
	}

	outputs := "CREATE TABLE outputs (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"user_id INTEGER," +
		"type TEXT, " +
		"credentials TEXT" +
		");"
	if _, err := c.Exec(outputs); err != nil {
		return err
	}

	targets := "create table targets (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"output_id INTEGER, " +
		"data TEXT, " +
		"FOREIGN KEY(user_id) REFERENCES users(id), " +
		"FOREIGN KEY(output_id) REFERENCES outputs(id)" +
		");"
	if _, err := c.Exec(targets); err != nil {
		return err
	}
	return nil
}

func LoadItem(id int64, url string, c *sql.DB, htmlPath string) error {
	contentIns := "INSERT INTO items_contents (url, item_id, html_path) VALUES($url, $item_id, $html_path)"

	res, err := http.Get(url)
	if err != nil {
		return err
	}
	log.Printf("Loading %s [%s]", url, "OK")
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	res.Body.Close()

	content, title, err := toReadableHtml(data)
	if err != nil {
		return err
	}
	// write html to path
	outPath := path.Join(htmlPath, title+".html")
	err = ioutil.WriteFile(outPath, content, 0644)
	if err != nil {
		return err
	}
	if !path.IsAbs(outPath) {
		outPath, _ = filepath.Abs(outPath)
	}
	itemArgs := []interface{}{url, id, outPath}

	if _, err = c.Exec(contentIns, itemArgs); err != nil {
		return err
	}

	return nil
}
