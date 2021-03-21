package feeds

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
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
		"author TEXT, " +
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

	/*
		insert := "INSERT INTO feeds (url, frequency, last_loaded) VALUES(?, ?, ?)"
		if _, err := c.Exec(insert, "https://www.parahumans.net/feed/", 120000000000, nil); err != nil {
			return err
		}
	*/

	users := "CREATE TABLE users ( id INTEGER PRIMARY KEY ASC );"
	if _, err := c.Exec(users); err != nil {
		return err
	}
	insertUsers := "INSERT INTO users (id) VALUES(?)"
	if _, err := c.Exec(insertUsers, 1); err != nil {
		return err
	}

	outputs := "CREATE TABLE outputs (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"user_id INTEGER," +
		"type TEXT, " +
		"credentials TEXT, " +
		"FOREIGN KEY(user_id) REFERENCES users(id)" +
		");"
	if _, err := c.Exec(outputs); err != nil {
		return err
	}

	targets := "create table targets (" +
		"id INTEGER PRIMARY KEY ASC, " +
		"output_id INTEGER, " +
		"data TEXT, " +
		"FOREIGN KEY(output_id) REFERENCES outputs(id)" +
		");"
	if _, err := c.Exec(targets); err != nil {
		return err
	}
	return nil
}

func LoadItem(it Item, c *sql.DB, htmlPath string) error {
	contentIns := "INSERT INTO items_contents (url, item_id, html_path) VALUES(?, ?, ?)"
	s, err := c.Prepare(contentIns)
	if err != nil {
		return err
	}
	defer s.Close()

	link := it.URL.String()
	res, err := http.Get(link)
	defer res.Body.Close()
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	content, title, err := toReadableHtml(data)
	if err != nil {
		return err
	}
	// write html to path
	outPath := path.Join(htmlPath, fmt.Sprintf("%s: %s.html", title, it.Title))
	err = ioutil.WriteFile(outPath, content, 0644)
	if err != nil {
		return err
	}
	if !path.IsAbs(outPath) {
		outPath, _ = filepath.Abs(outPath)
	}

	if _, err = s.Exec(link, it.ID, outPath); err != nil {
		return err
	}

	return nil
}

func GetFeeds(c *sql.DB) ([]Feed, error) {
	sql := "SELECT id, url, frequency, last_loaded, title, author FROM feeds"
	s, err := c.Query(sql)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]Feed, 0)
	for s.Next() {
		var link string
		f := Feed{}
		s.Scan(&f.ID, &link, &f.Frequency, &f.Updated, &f.Title, &f.Author)
		if f.URL, err = url.Parse(link); err == nil {
			all = append(all, f)
		}
	}
	return all, nil
}

func GetNonFetchedItems(c *sql.DB) ([]Item, error) {
	sql := "SELECT items.id, feeds.title as feed_title, items.title as title, items.url FROM items INNER JOIN feeds ON feeds.id = items.feed_id LEFT JOIN items_contents ON items_contents.item_id = items.id WHERE items_contents.id IS NULL group by items.id"
	s, err := c.Query(sql)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]Item, 0)
	for s.Next() {
		it := Item{}
		var link string

		err := s.Scan(&it.ID, &it.Feed.Title, &it.Title, &link)
		if err != nil {
			continue
		}
		it.URL, _ = url.Parse(link)

		all = append(all, it)
	}

	return all, nil
}

func GetNonDispatchedItemContents(c *sql.DB) ([]Content, error) {
	sql := "SELECT items_contents.id, feeds.title, items.title, items.author, mobi_path " +
		"FROM items_contents " +
		"INNER JOIN items ON items.id = items_contents.item_id " +
		"INNER JOIN feeds ON feeds.id = items.feed_id WHERE items_contents.dispatched != 1"

	s, err := c.Query(sql)
	defer s.Close()
	if err != nil {
		return nil, err
	}
	all := make([]Content, 0)
	for s.Next() {
		cont := Content{}
		s.Scan(&cont.Item.ID, &cont.Item.Feed.Title, &cont.Item.Title, &cont.Item.Author, &cont.MobiPath)
		all = append(all, cont)
	}
	return all, nil
}

func GetContentsForMobi(c *sql.DB) ([]Content, error) {
	sql := "SELECT items_contents.id, feeds.title, items.title, items.author, html_path " +
		"FROM items_contents " +
		"INNER JOIN items ON items.id = items_contents.item_id " +
		"INNER JOIN feeds ON feeds.id = items.feed_id WHERE mobi_path IS NULL"

	s, err := c.Query(sql)
	if err != nil {
		return nil, err
	}
	all := make([]Content, 0)
	for s.Next() {
		cont := Content{}
		s.Scan(&cont.ID, &cont.Item.Feed.Title, &cont.Item.Title, &cont.Item.Author, &cont.HTMLPath)
		all = append(all, cont)
	}

	return all, nil
}
