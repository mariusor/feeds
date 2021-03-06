package feeds

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
		created TEXT,
		FOREIGN KEY(item_id) REFERENCES items(id),
		CONSTRAINT contents_uindex UNIQUE (item_id, type)
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

	destinations := `CREATE TABLE destinations (
		id INTEGER PRIMARY KEY ASC,
		type TEXT,
		credentials TEXT,
		created TEXT,
		flags INT DEFAULT 0
	);`
	if _, err := c.Exec(destinations); err != nil {
		return err
	}

	dispatched := `create table dispatched (
		id INTEGER PRIMARY KEY ASC,
		destination_id int,
		item_id int,
		last_try TEXT,
		last_status int,
		last_message text,
		flags INT DEFAULT 0,
		FOREIGN KEY(item_id) REFERENCES items(id),
		FOREIGN KEY(destination_id) REFERENCES destinations(id) ON DELETE CASCADE,
		CONSTRAINT item_destination_uindex UNIQUE (item_id, destination_id)
	);`
	if _, err := c.Exec(dispatched); err != nil {
		return err
	}

	subscriptions := `CREATE TABLE subscriptions (
		id INTEGER PRIMARY KEY ASC,
		feed_id int,
		destination_id int,
		created TEXT,
		flags INT DEFAULT 0,
		FOREIGN KEY(feed_id) REFERENCES feeds(id),
		FOREIGN KEY(destination_id) REFERENCES destinations(id) ON DELETE CASCADE,
		CONSTRAINT feed_destination_uindex UNIQUE (feed_id, destination_id)
	);`
	if _, err := c.Exec(subscriptions); err != nil {
		return err
	}
	/*
	// We disable these tables for now
	insertUsers := `INSERT INTO users (id) VALUES(?);`
	if _, err := c.Exec(insertUsers, 1); err != nil {
		return err
	}

	// table dispatched holds the details of the local application configuration for the Target it represents
	*/

	return nil
}

func LoadItem(it *Item, c *sql.DB, basePath string) (bool, error) {
	contentIns := `INSERT INTO contents (item_id, path, type) VALUES(?, ?, ?);`
	s1, err := c.Prepare(contentIns)
	if err != nil {
		return false, err
	}
	defer s1.Close()

	itemUpd := `UPDATE items SET last_loaded = ?, last_status = ? WHERE id = ?`
	s2, err := c.Prepare(itemUpd)
	if err != nil {
		return false, err
	}

	link := it.URL.String()
	if !path.IsAbs(basePath) {
		basePath, _ = filepath.Abs(basePath)
	}

	if len(it.Content) == 0 {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, link, nil)
		if err != nil {
			return false, err
		}
		req.Header.Add("User-Agent", "feed-sync//1.0")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, err
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return false, fmt.Errorf("invalid response received %s", res.Status)
		}
		it.Status = res.StatusCode
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false, err
		}

		// write received html to path
		feedPath := path.Join(basePath, HtmlDir, strings.TrimSpace(it.Feed.Title))
		articlePath := path.Join(feedPath, it.Path("html"))
		if _, err := os.Stat(feedPath); err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(feedPath, 0755); err != nil {
				return false, err
			}
		}

		if avgSize := feedItemsAverageSize(feedPath); avgSize > 0 && len(data) < avgSize/7 {
			return false, errors.New("file size is smaller than 14% of average of existing ones")
		}

		if err = ioutil.WriteFile(articlePath, data, 0644); err != nil {
			return false, err
		}
		if _, err = s1.Exec(it.ID, sql.NullString{String: articlePath, Valid: len(articlePath) > 0}, "raw"); err != nil {
			return false, err
		}
	}

	if _, err = s2.Exec(time.Now().UTC().Format(time.RFC3339), it.Status, it.ID); err != nil {
		return false, err
	}
	return true, nil
}

func feedItemsAverageSize(path string) int {
	var sum, cnt int64

	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		sum += info.Size()
		cnt++
		return nil
	})
	if sum == 0 || cnt == 0 {
		return -1
	}
	return int(sum/cnt)
}

func GetFeeds(c *sql.DB) ([]Feed, error) {
	sel := `SELECT id, title, author, frequency, last_loaded, url, flags FROM feeds where flags != ?`
	s, err := c.Query(sel, FlagsDisabled)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]Feed, 0)
	for s.Next() {
		var (
			id, flags     int
			freq          sql.NullInt32
			title, auth   string
			link, updated sql.NullString
		)
		s.Scan(&id, &title, &auth, &freq, &updated, &link, &flags)
		f := Feed{
			ID:        id,
			Title:     title,
			Author:    auth,
			Frequency: time.Duration(freq.Int32) * time.Second,
			Flags: flags,
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
	sel := `
SELECT items.id, items.feed_index, feeds.id, feeds.title AS feed_title, items.title AS title, items.url, c.id, c.type, c.path 
FROM items
INNER JOIN feeds ON feeds.id = items.feed_id
LEFT JOIN contents c ON items.id = c.item_id AND c.type  = 'raw'
WHERE c.id IS NULL and feeds.flags != ? ORDER BY items.feed_index ASC;`
	s, err := c.Query(sel, FlagsDisabled)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]Item, 0)
	for s.Next() {
		it := Item{}
		var (
			link                 string
			feedIndex, contentId sql.NullInt32
			cTyp, cPath          sql.NullString
		)

		err := s.Scan(&it.ID, &feedIndex, &it.Feed.ID, &it.Feed.Title, &it.Title, &link, &contentId, &cTyp, &cPath)
		if err != nil {
			continue
		}
		it.URL, _ = url.Parse(link)
		if feedIndex.Valid {
			it.FeedIndex = int(feedIndex.Int32)
		}
		if contentId.Valid {
			it.Content = make(map[string]Content)
			it.Content["raw"] = Content{
				ID:   int(contentId.Int32),
				Type: cTyp.String,
				Path: cPath.String,
			}
		}

		all = append(all, it)
	}

	return all, nil
}

type DispatchItem struct {
	ID          int
	LastStatus  bool
	LastMessage string
	Flags       int
	Item        Item
	Destination Destination
}

var subscriptionBackPeriod = 7 * 24 * time.Hour

func GetNonDispatchedItemContentsForDestination(c *sql.DB) ([]DispatchItem, error) {
	wheres := make([]string, 0)
	params := make([]interface{}, 0)
	for typ, t := range ValidTargets {
		wheres = append(wheres, fmt.Sprintf("d.type = '%s' AND c.type IN ('%s')", typ, strings.Join(t.ValidContentTypes(), "', '")))
	}
	sel := fmt.Sprintf(`SELECT t.id, c.id, i.id, f.title, i.title, i.author, i.url, c.path, c.type, d.id, d.type, d.credentials FROM items i
INNER JOIN feeds f ON i.feed_id = f.id
INNER JOIN subscriptions s ON f.id = s.feed_id
INNER JOIN destinations d ON d.id = s.destination_id
INNER JOIN contents c ON c.item_id = i.id AND (%s)
LEFT JOIN dispatched t ON t.item_id = i.id AND t.destination_id = d.id  
WHERE date(i.last_loaded) > date(s.created, '-%f hour') AND (t.id IS NULL OR (t.id IS NOT NULL AND t.last_status = 0))
GROUP BY i.id, d.type, d.id ORDER BY i.id;`, strings.Join(wheres, " OR "), subscriptionBackPeriod.Hours())

	s, err := c.Query(sel, params...)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]DispatchItem, 0)
	for s.Next() {
		var (
			it                        = Item{Content: make(map[string]Content)}
			dest                      = Destination{}
			contType, contPath, itURL string
			contID                    int
			targetID                  sql.NullInt32
		)
		err := s.Scan(&targetID, &contID, &it.ID, &it.Feed.Title, &it.Title, &it.Author, &itURL, &contPath, &contType, &dest.ID, &dest.Type, &dest.Credentials)
		if err != nil {
			continue
		}
		it.URL, _ = url.Parse(itURL)
		it.Content[contType] = Content{ID: contID, Path: contPath, Type: contType}
		dd := DispatchItem{
			Item:        it,
			Destination: dest,
		}
		if targetID.Valid {
			dd.ID = int(targetID.Int32)
		}
		all = append(all, dd)
	}
	return all, nil
}

func GetContentsForEbook(c *sql.DB, types ...string) ([]Item, error) {
	joins := make([]string, 0)
	cols := make([]string, 0)
	wheres := make([]string, 0)
	for _, typ := range types {
		cols = append(cols, fmt.Sprintf("%s.path", typ))
		joins = append(joins, fmt.Sprintf("LEFT JOIN contents AS %s ON items.id = %s.item_id AND %s.type = '%s' \n", typ, typ, typ, typ))
		wheres = append(wheres, fmt.Sprintf("%s.path IS NULL", typ))

	}

	sel := `SELECT items.id, items.feed_index, feeds.title, items.title, items.author, raw.id, raw.type, raw.path, %s FROM items
	INNER JOIN feeds ON feeds.id = items.feed_id
	INNER JOIN contents AS raw ON items.id = raw.item_id AND raw.type = 'raw'
%s WHERE %s`
	q := fmt.Sprintf(sel, strings.Join(cols, ", "), strings.Join(joins, ""), strings.Join(wheres, " OR "))
	s1, err := c.Query(q)
	if err != nil {
		return nil, err
	}
	defer s1.Close()

	all := make(map[int]Item)
	itemIds := make([]string, 0)
	for s1.Next() {
		var (
			id                       int
			feedTitle, title, author string
			feedIndex, rawId         sql.NullInt32
			it                       Item
			ok                       bool
			rawType, rawPath         sql.NullString
		)
		params := []interface{}{&id, &feedIndex, &feedTitle, &title, &author, &rawId, &rawType, &rawPath}
		paths := make(map[string]sql.NullString)
		for _, typ := range types {
			params = append(params, interface{}(paths[typ]))
		}
		s1.Scan(params...)
		if it, ok = all[id]; !ok || it.ID != id {
			it = Item{
				ID:     id,
				Title:  title,
				Author: author,
				Feed:   Feed{Title: feedTitle},
			}
			if feedIndex.Valid {
				it.FeedIndex = int(feedIndex.Int32)
			}
			if rawId.Valid {
				it.Content = make(map[string]Content)
				it.Content["raw"] = Content{ID: int(rawId.Int32), Type: rawType.String, Path: rawPath.String}
			}
			for _, typ := range types {
				if c, ok := paths[typ]; ok && c.Valid {
					it.Content[typ] = Content{Type: typ, Path: c.String}
				}
			}
		}
		itemIds = append(itemIds, fmt.Sprintf("%d", id))
		all[it.ID] = it
	}
	contWhere := strings.Join(itemIds, ", ")
	selCont := fmt.Sprintf(`SELECT id, item_id, path, type from contents where item_id in (%s) and type != 'raw';`, contWhere)
	s2, err := c.Query(selCont)
	if err != nil {
		return nil, err
	}
	defer s2.Close()

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
	sel := `SELECT items.id, feeds.title, items.title, items.author, items.feed_index FROM items 
INNER JOIN feeds ON feeds.id = items.feed_id 
WHERE items.feed_id = ? ORDER BY items.feed_index ASC;`

	s, err := c.Query(sel, f.ID)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	all := make([]Item, 0)
	itemIds := make([]string, 0)
	for s.Next() {
		var (
			id                       int
			feedIndex                sql.NullInt32
			feedTitle, title, author string
		)
		s.Scan(&id, &feedTitle, &title, &author, &feedIndex)
		it := Item{
			ID:     id,
			Title:  title,
			Author: author,
			Feed:   Feed{Title: feedTitle},
		}
		if feedIndex.Valid {
			it.FeedIndex = int(feedIndex.Int32)
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
	defer s2.Close()

	for s2.Next() {
		var (
			inx, id, itemId int
			path, typ       string
			item            Item
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

type Services map[string]DestinationTarget

type User struct {
	ID       int
	Flags    int
	Services Services
}

type Destination struct {
	ID          int
	Type        string
	Credentials []byte
	Created     time.Time
	Flags       int
}

func loadMyKindleDestination(c *sql.DB, d MyKindleDestination) (*Destination, error) {
	sel := `SELECT id, type, credentials, flags FROM destinations 
WHERE type = ? AND json_extract(credentials, '$.to') = ?`
	s, err := c.Query(sel, d.Type(), d.To)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	dd := Destination{}
	for s.Next() {
		s.Scan(&dd.ID, &dd.Type, &dd.Credentials, &dd.Flags)
	}
	if dd.ID > 0 {
		return &dd, nil
	}
	return nil, nil
}

func loadPocketDestination(c *sql.DB, d PocketDestination) (*Destination, error) {
	sel := `SELECT id, type, credentials, flags FROM destinations 
WHERE type = ? AND json_extract(credentials, '$.username') = ?`
	s, err := c.Query(sel, d.Type(), d.Username)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	dd := Destination{}
	for s.Next() {
		s.Scan(&dd.ID, &dd.Type, &dd.Credentials, &dd.Flags)
	}
	if dd.ID > 0 {
		return &dd, nil
	}
	return nil, nil
}

func loadDestination(c *sql.DB, d DestinationTarget) (*Destination, error) {
	switch dd := d.(type) {
	case PocketDestination:
		return loadPocketDestination(c, dd)
	case MyKindleDestination:
		return loadMyKindleDestination(c, dd)
	}
	return nil, errors.New("invalid destination")
}

func insertDestination(c *sql.DB, d Destination) (*Destination, error) {
	sql := `INSERT INTO destinations (type, credentials, flags, created) VALUES(?, ?, ?);`
	r, err := c.Exec(sql, d.Type, d.Credentials, d.Flags, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	if id, err := r.LastInsertId(); err == nil {
		d.ID = int(id)
	}
	return &d, nil
}

func updateDestination(c *sql.DB, d Destination) (*Destination, error) {
	sql := `UPDATE destinations SET type = ?, credentials = ?, flags = ? WHERE id = ?`
	if _, err := c.Exec(sql, d.Type, d.Credentials, d.Flags, d.ID); err != nil {
		return nil, err
	}
	return &d, nil
}

func SaveDestination(c *sql.DB, d DestinationTarget) (*Destination, error) {
	creds, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal credentials: %w", err)
	}

	dd, err := loadDestination(c, d)
	if err != nil {
		return nil, err
	}

	if dd == nil {
		dd = &Destination{
			Type:        d.Type(),
			Credentials: creds,
			Flags:       FlagsNone,
		}
		return insertDestination(c, *dd)
	}
	dd.Credentials = creds
	return updateDestination(c, *dd)
}

type Subscription struct {
	ID          int
	Flags       int
	Created     time.Time
	Destination Destination
	Feed        Feed
}

func SaveSubscriptions(c *sql.DB, d Destination, feeds ...Feed) error {
	ins := `INSERT INTO subscriptions (feed_id, destination_id, created) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;`
	s, err := c.Prepare(ins)
	if err != nil {
		return err
	}
	d.Created = time.Now().UTC()
	for _, f := range feeds {
		if f.URL == nil {
			continue
		}
		if _, err := s.Exec(f.ID, d.ID, d.Created.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

func insertTarget(c *sql.DB, t DispatchItem) error {
	sql := `INSERT INTO dispatched (destination_id, item_id, flags, last_try, last_status, last_message) VALUES(?, ?, ?, ?, ?);`
	if _, err := c.Exec(sql, t.Destination.ID, t.Item.ID, t.Flags, time.Now().UTC().Format(time.RFC3339), t.LastStatus, t.LastMessage); err != nil {
		return err
	}
	return nil
}

func updateTarget(c *sql.DB, t DispatchItem) error {
	sql := `UPDATE dispatched SET last_status = ?, last_message = ?, flags = ?, last_try = ? WHERE id = ?`
	if _, err := c.Exec(sql, t.LastStatus, t.LastMessage, t.Flags, time.Now().UTC().Format(time.RFC3339), t.ID); err != nil {
		return err
	}
	return nil
}

func loadTarget(c *sql.DB, t DispatchItem) (*DispatchItem, error) {
	sql := `SELECT id, last_status, last_message, flags FROM dispatched WHERE destination_id = ? AND item_id = ?`
	s, err := c.Query(sql, t.Destination.ID, t.Item.ID)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	for s.Next() {
		s.Scan(&t.ID, &t.LastStatus, &t.LastMessage, &t.Flags)
	}
	if t.ID > 0 {
		return &t, nil
	}
	return nil, nil
}

func SaveTarget(c *sql.DB, tt DispatchItem) error {
	t, err := loadTarget(c, tt)
	if err != nil {
		return err
	}

	if t == nil {
		return insertTarget(c, tt)
	}
	t.Flags = tt.Flags
	t.LastMessage = tt.LastMessage
	t.LastStatus = tt.LastStatus
	return updateTarget(c, *t)
}
