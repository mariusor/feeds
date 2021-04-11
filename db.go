package feeds

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const DBFilePath = "feeds.db"

const (
	FlagsDisabled = 1 << iota

	FlagsNone = 0
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
		id INTEGER PRIMARY KEY ASC AUTOINCREMENT,
		url TEXT,
		title TEXT,
		author TEXT,
		frequency REAL,
		last_loaded TEXT,
		last_status INTEGER,
		flags INTEGER DEFAULT 0
	);`
	if _, err := c.Exec(feeds); err != nil {
		return err
	}

	items := `CREATE TABLE items (
		id INTEGER PRIMARY KEY ASC AUTOINCREMENT,
		url TEXT,
		feed_id INTEGER,
		guid TEXT,
		title TEXT,
		author TEXT,
		feed_index INTEGER,
		published_date TEXT,
		last_loaded TEXT,
		last_status INTEGER,
		FOREIGN KEY(feed_id) REFERENCES feeds(id)
	);`
	if _, err := c.Exec(items); err != nil {
		return err
	}

	contents := `CREATE TABLE contents (
		id INTEGER PRIMARY KEY ASC AUTOINCREMENT,
		item_id INTEGER,
		path TEXT,
		type TEXT,
		FOREIGN KEY(item_id) REFERENCES items(id),
		constraint contents_uindex unique (item_id, type)
	);`
	if _, err := c.Exec(contents); err != nil {
		return err
	}

	users := `CREATE TABLE users (
		id INTEGER PRIMARY KEY ASC,
		raw TEXT,
		flags INTEGER
	);`
	if _, err := c.Exec(users); err != nil {
		return err
	}
	/*
	// We disable these tables for now
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

	// table targets holds the details of the local application configuration for the service it represents
	targets := `create table targets (
		id INTEGER PRIMARY KEY ASC,
		service TEXT,
		data TEXT,
		flags INT DEFAULT 0
	);`
	if _, err := c.Exec(targets); err != nil {
		return err
	}
	*/

	return nil
}

func LoadItem(it Item, c *sql.DB, basePath string) (int, error) {
	contentIns := `INSERT INTO contents (item_id, path, type) VALUES(?, ?, ?)`
	s1, err := c.Prepare(contentIns)
	if err != nil {
		return 0, err
	}
	defer s1.Close()

	itemUpd := `UPDATE items SET last_loaded = ?, last_status = ? WHERE id = ?`
	s2, err := c.Prepare(itemUpd)
	if err != nil {
		return 0, err
	}

	link := it.URL.String()
	if !path.IsAbs(basePath) {
		basePath, _ = filepath.Abs(basePath)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, link, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Add("User-Agent", "feed-sync//1.0")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return res.StatusCode, err
		}

		// write received html to path
		articlePath := path.Join(
			basePath,
			HtmlDir,
			strings.TrimSpace(it.Feed.Title),
			it.Path("html"),
		)
		if _, err := os.Stat(path.Dir(articlePath)); err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(path.Dir(articlePath), 0755); err != nil {
				return res.StatusCode, err
			}
		}
		if err = ioutil.WriteFile(articlePath, data, 0644); err != nil {
			return res.StatusCode, err
		}

		_, err = s1.Exec(it.ID, sql.NullString{ String: articlePath, Valid:  len(articlePath) > 0 }, "raw")
		if err != nil {
			return res.StatusCode, err
		}

		content, _, err := toReadableHtml(data)
		if err != nil {
			return res.StatusCode, err
		}
		outPath := path.Join(
			basePath,
			OutputDir,
			strings.TrimSpace(it.Feed.Title),
			"html",
			it.Path("html"),
		)
		if _, err := os.Stat(path.Dir(outPath)); err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(path.Dir(outPath), 0755); err != nil {
				return res.StatusCode, err
			}
		}
		if err = ioutil.WriteFile(outPath, content, 0644); err != nil {
			return res.StatusCode, err
		}
		_, err = s1.Exec(it.ID, sql.NullString{ String: outPath, Valid:  len(outPath) > 0 }, "html")
		if err != nil {
			return res.StatusCode, err
		}
	}
	if _, err = s2.Exec(time.Now().UTC().Format(time.RFC3339), res.StatusCode, it.ID); err != nil {
		return res.StatusCode, err
	}
	return res.StatusCode, nil
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
	sql := `
SELECT items.id, items.feed_index, feeds.title as feed_title, items.title as title, items.url FROM items 
    INNER JOIN feeds ON feeds.id = items.feed_id 
WHERE items.last_loaded IS NULL group by items.id order by items.feed_index asc`
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

func GetNonDispatchedItemContents(c *sql.DB) ([]Item, error) {
	sql := `SELECT contents.id, items.id, feeds.title, items.title, items.author, path, type 
		FROM contents 
		INNER JOIN items ON items.id = contents.item_id 
		INNER JOIN feeds ON feeds.id = items.feed_id WHERE contents.dispatched != 1;`

	s, err := c.Query(sql)
	defer s.Close()
	if err != nil {
		return nil, err
	}
	all := make([]Item, 0)
	for s.Next() {
		var (
			cont   = Item{}
			contID, contType, contPath string
		)
		s.Scan(&cont.ID, &contID, &cont.Feed.Title, &cont.Title, &cont.Author, &contPath, &contType)
		all = append(all, cont)
	}
	return all, nil
}

func GetContentsForEbook(c *sql.DB) ([]Item, error) {
	sql := `SELECT items.id, items.feed_index, feeds.title, items.title, items.author FROM items
    INNER JOIN feeds ON feeds.id = items.feed_id
    INNER JOIN contents AS html ON items.id = html.item_id AND html.type = 'html'
    LEFT JOIN contents AS epub ON items.id = epub.item_id AND epub.type = 'epub'
    LEFT JOIN contents AS mobi ON items.id = mobi.item_id AND mobi.type = 'mobi'
WHERE epub.path IS NULL OR mobi.path IS NULL; `

	s1, err := c.Query(sql)
	if err != nil {
		return nil, err
	}
	all := make(map[int]Item)
	itemIds := make([]string, 0)
	for s1.Next() {
		var (
			id, feedIndex                                int
			feedTitle, title, author string
			cont                                         Item
			ok                                           bool
		)
		s1.Scan(&id, &feedIndex, &feedTitle, &title, &author)
		if cont, ok = all[id]; !ok || cont.ID != id {
			cont = Item{
				ID: id, 
				FeedIndex: feedIndex,
				Title: title,
				Author: author,
				Feed: Feed{Title: feedTitle},
			}
		}
		itemIds = append(itemIds, fmt.Sprintf("%d", id))
		all[cont.ID] = cont
	}
	contWhere := strings.Join(itemIds, ", ")
	selCont := fmt.Sprintf(`SELECT id, item_id, path, type from contents where item_id in (%s) and type != 'raw';`, contWhere)
	s2, err := c.Query(selCont)
	if err != nil {
		return nil, err
	}
	for s2.Next() {
		var (
			id, itemId int
			path, typ  string
			ok         bool
		)
		s2.Scan(&id, &itemId, &path, &typ)
		item, ok := all[itemId]
		if !ok {
			// TODO(marius) missing item, error
		}
		if item.Content == nil {
			item.Content = make(map[string]Content)
		}
		item.Content[typ] = Content{ID: id, Path: path, Type: typ}
		all[itemId] = item
	}

	result := make([]Item, 0)
	for _, it := range all {
		result = append(result, it)
	}
	return result, nil
}

func GetItemsByFeedAndType(c *sql.DB, f Feed, ext string) ([]Item, error) {
	sql := `SELECT items.id, feeds.title, items.title, items.author, items.feed_index FROM items 
INNER JOIN feeds ON feeds.id = items.feed_id 
WHERE items.feed_id = ? ORDER BY items.feed_index ASC;`

	s, err := c.Query(sql, f.ID)
	if err != nil {
		return nil, err
	}
	all := make([]Item, 0)
	itemIds := make([]string, 0)
	for s.Next() {
		var (
			id, feedIndex            int
			feedTitle, title, author string
		)
		s.Scan(&id, &feedTitle, &title, &author, &feedIndex)
		it := Item{
			ID: id,
			FeedIndex: feedIndex,
			Title: title,
			Author: author,
			Feed: Feed{Title: feedTitle},
		}
		itemIds = append(itemIds, fmt.Sprintf("%d", id))
		all = append(all, it)
	}

	contWhere := strings.Join(itemIds, ", ")
	selCont := fmt.Sprintf(`SELECT id, item_id, path, type from contents where item_id in (%s);`, contWhere)
	s2, err := c.Query(selCont)
	if err != nil {
		return nil, err
	}
	for s2.Next() {
		var (
			inx, id, itemId int
			path, typ  string
			item Item
		)
		s2.Scan(&id, &itemId, &path, &typ)
		for i, it := range all {
			if it.ID == itemId {
				item = it
				inx = i
			}
		}
		if item.Content == nil {
			item.Content = make(map[string]Content)
		}
		item.Content[typ] = Content{ID: id, Path: path, Type: typ}
		all[inx] = item
	}
	return all, nil
}

type Services map[string]TargetDestination

type User struct {
	ID       int
	Flags    int
	Services Services
}

func LoadUserByService(c *sql.DB, username, service string) (*User, error) {
	sql := `SELECT id, raw from users where json_extract("raw", '$.services.?.id') = ?`
	s, err := c.Query(sql, service, username)
	if err != nil {
		return nil, err
	}
	all := make([]User, 0)
	for s.Next() {
		user := User{}
		raw := make([]byte, 0)
		s.Scan(&user.ID, &raw)
		all = append(all, user)
	}

	if len(all) > 1 {
		return nil, fmt.Errorf("too many users")
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("user not found")
	}
	return &all[0], nil
}

func SaveUserService(c *sql.DB, u User) error {
	return nil
}
