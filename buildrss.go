package feeds

import (
	"bytes"
	"database/sql"
	"net/url"

	"git.sr.ht/~ghost08/ratt"
	"gopkg.in/yaml.v2"
)

func LoadRattConf(c *sql.DB, url *url.URL) (ratt.Selectors, error) {
	var sel ratt.Selectors
	sql := `SELECT selectors FROM ratt_selectors WHERE url like ?`
	s, err := c.Query(sql, url.String())
	if err != nil {
		return sel, err
	}
	defer s.Close()

	var rawSelectors []byte
	s.Scan(&rawSelectors)
	if err := yaml.NewDecoder(bytes.NewReader(rawSelectors)).Decode(&sel); err != nil {
		return sel, err
	}
	return sel, nil
}

func ToFeed(c *sql.DB, url *url.URL, body []byte) ([]byte, error) {
	sel, err := LoadRattConf(c, url)
	if err != nil {
		return nil, err
	}

	feed, err := sel.ConstructFeedFromURL(url)
	if err != nil {
		return nil, err
	}

	data, err := feed.ToRss()
	if err != nil {
		return nil, err
	}
	return []byte(data), err
}
