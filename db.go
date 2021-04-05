package feeds

import (
	"database/sql"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const DBFilePath = "feeds.db"

const (
	FlagsNone     = iota
	FlagsDisabled = 1 << iota
)

func DB(basePath string) (*sql.DB, error) {
	dbPath := path.Join(basePath, DBFilePath)
	bootstrap := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		bootstrap = true
	}
	db, err := sql.Open("sqlite", dbPath)
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
	feeds := `CREATE TABLE feeds (
		id INTEGER PRIMARY KEY ASC,
		url TEXT,
		title TEXT,
		author TEXT,
		frequency REAL,
		last_loaded DATETIME,
		last_status INTEGER,
		flags INTEGER DEFAULT 0
	)`
	if _, err := c.Exec(feeds); err != nil {
		return err
	}

	items := `CREATE TABLE items (
		id INTEGER PRIMARY KEY ASC,
		url TEXT,
		feed_id INTEGER,
		guid TEXT,
		title TEXT,
		author TEXT,
		feed_index INTEGER,
		published_date DATETIME,
		last_loaded DATETIME,
		last_status INTEGER,
		FOREIGN KEY(feed_id) REFERENCES feeds(id)
	)`
	if _, err := c.Exec(items); err != nil {
		return err
	}

	contents := `CREATE TABLE items_contents (
		id INTEGER PRIMARY KEY ASC,
		url TEXT,
		item_id INTEGER,
		html_path TEXT,
		mobi_path TEXT,
		epub_path TEXT,
		dispatched INTEGER DEFAULT 0,
		FOREIGN KEY(item_id) REFERENCES items(id)
	)`
	if _, err := c.Exec(contents); err != nil {
		return err
	}

	users := `CREATE TABLE users (
		id INTEGER PRIMARY KEY ASC,
		raw TEXT
	);`
	if _, err := c.Exec(users); err != nil {
		return err
	}
	insertUsers := `INSERT INTO users (id) VALUES(?);`
	if _, err := c.Exec(insertUsers, 1); err != nil {
		return err
	}

	outputs := `CREATE TABLE outputs (
		id INTEGER PRIMARY KEY ASC,
		user_id INTEGER,
		type TEXT,
		credentials TEXT,
		flags INT DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	if _, err := c.Exec(outputs); err != nil {
		return err
	}

	targets := `create table targets (
		id INTEGER PRIMARY KEY ASC,
		output_id INTEGER,
		data TEXT,
		flags INT DEFAULT 0,
		FOREIGN KEY(output_id) REFERENCES outputs(id)
	);`
	if _, err := c.Exec(targets); err != nil {
		return err
	}
	return nil
}

func LoadItem(it Item, c *sql.DB, htmlPath string) error {
	contentIns := `INSERT INTO items_contents (url, item_id, html_path) VALUES(?, ?, ?)`
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

	content, _, err := toReadableHtml(data)
	if err != nil {
		return err
	}
	// write html to path
	outPath := path.Join(htmlPath, it.Path("html"))
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
	sel := `SELECT id, title, author, frequency, last_loaded, url FROM feeds where flags != ?`
	s, err := c.Query(sel, FlagsDisabled)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]Feed, 0)
	for s.Next() {
		var (
			id            int
			freq          sql.NullInt32
			title, auth   string
			link, updated sql.NullString
		)
		s.Scan(&id, &title, &auth, &freq, &updated, &link)
		f := Feed{
			ID:        id,
			Title:     title,
			Author:    auth,
			Frequency: time.Duration(freq.Int32) * time.Second,
		}
		if updated.Valid {
			f.Updated, _ = time.Parse(time.RFC3339Nano, updated.String)
		}
		if link.Valid {
			f.URL, _ = url.Parse(link.String)
		}
		all = append(all, f)
	}
	return all, nil
}

func GetNonFetchedItems(c *sql.DB) ([]Item, error) {
	sql := `SELECT items.id, items.feed_index, feeds.title as feed_title, items.title as title, items.url 
FROM items 
    INNER JOIN feeds ON feeds.id = items.feed_id 
    LEFT JOIN items_contents ON items_contents.item_id = items.id 
WHERE items_contents.id IS NULL group by items.id order by items.feed_index asc`
	s, err := c.Query(sql)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]Item, 0)
	for s.Next() {
		it := Item{}
		var link string

		err := s.Scan(&it.ID, &it.FeedIndex, &it.Feed.Title, &it.Title, &link)
		if err != nil {
			continue
		}
		it.URL, _ = url.Parse(link)

		all = append(all, it)
	}

	return all, nil
}

func GetNonDispatchedItemContents(c *sql.DB) ([]Content, error) {
	sql := "SELECT items_contents.id, items.id, feeds.title, items.title, items.author, mobi_path, epub_path " +
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
		s.Scan(&cont.ID, &cont.Item.ID, &cont.Item.Feed.Title, &cont.Item.Title, &cont.Item.Author, &cont.MobiPath, &cont.EPubPath)
		all = append(all, cont)
	}
	return all, nil
}

func GetContentsForEbook(c *sql.DB) ([]Content, error) {
	sql := `SELECT items_contents.id, items.id, feeds.title, items.title, items.author, html_path, mobi_path, epub_path 
	FROM items_contents 
INNER JOIN items ON items.id = items_contents.item_id 
INNER JOIN feeds ON feeds.id = items.feed_id WHERE mobi_path IS NULL OR epub_path IS NULL;`

	s, err := c.Query(sql)
	if err != nil {
		return nil, err
	}
	all := make([]Content, 0)
	for s.Next() {
		cont := Content{}
		s.Scan(&cont.ID, &cont.Item.ID, &cont.Item.Feed.Title, &cont.Item.Title, &cont.Item.Author, &cont.HTMLPath, &cont.MobiPath, &cont.EPubPath)
		all = append(all, cont)
	}

	return all, nil
}

func GetContentsByFeed(c *sql.DB, f Feed) ([]Content, error) {
	sql := `SELECT 
       items_contents.id,
       items.id,
       feeds.title,
       items.title,
       items.author,
       html_path,
       mobi_path,
       epub_path 
	FROM items_contents 
INNER JOIN items ON items.id = items_contents.item_id 
INNER JOIN feeds ON feeds.id = items.feed_id WHERE items.feed_id = ?;`

	s, err := c.Query(sql, f.ID)
	if err != nil {
		return nil, err
	}
	all := make([]Content, 0)
	for s.Next() {
		cont := Content{}
		s.Scan(&cont.ID, &cont.Item.ID, &cont.Item.Feed.Title, &cont.Item.Title, &cont.Item.Author, &cont.HTMLPath, &cont.MobiPath, &cont.EPubPath)
		all = append(all, cont)
	}

	return all, nil
}
