package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/766b/mobi"
	"github.com/SlyMarbo/rss"
	"github.com/mariusor/go-readability"
	"github.com/mxk/go-sqlite/sqlite3"
	"golang.org/x/net/html"
	//"golang.org/x/net/html/atom"
)

type tag string

const (
	meta   = "meta"
	script = "script"
	style  = "style"
	iframe = "iframe"
	link   = "link"
	br     = "br"
	hr     = "hr"
	href   = "href"
)

var nonContentTags []string = []string{meta, script, style, iframe, link}
var allowEmptyTag []string = []string{hr, br}

var allowedAttrs []string = []string{href, style}

func inArr(search string, arr []string) bool {
	for _, elem := range nonContentTags {
		if elem == search {
			return true
		}
	}
	return false
}

func sanitize(text string) string {
	re := regexp.MustCompilePOSIX("[\t\n\f\r ]+")

	return strings.Trim(re.ReplaceAllString(text, "\n"), " \n\t\r")
}

func parseHtml(r io.Reader) {
	d := html.NewTokenizer(r)
	skipContent := false
	for {
		// token type
		tokenType := d.Next()
		if tokenType == html.ErrorToken {
			break
		}
		token := d.Token()

		switch tokenType {
		case html.StartTagToken: // <tag>
			if inArr(token.Data, nonContentTags) {
				skipContent = true
				continue
			}
			fmt.Printf("<%s", token.Data)
			for _, attr := range token.Attr {
				if inArr(attr.Key, allowedAttrs) {
					fmt.Printf(" %s=%q", attr.Key, attr.Val)
				}
			}
			fmt.Printf(">")
		case html.TextToken: // text between start and end tag
			if skipContent {
				continue
			}
			fmt.Printf("%s", sanitize(token.Data))
		case html.EndTagToken: // </tag>
			if inArr(token.Data, nonContentTags) {
				skipContent = false
				continue
			}
			fmt.Printf("</%s>\n", token.Data)
		case html.SelfClosingTagToken: // <tag/>
			if inArr(token.Data, nonContentTags) {
				continue
			}
			if !inArr(token.Data, allowEmptyTag) {
				continue
			}
			fmt.Printf("<%s/>\n", token.Data)
		}
	}
}
func toReadableHtml(content []byte) (string, string, error) {
	var err error
	var doc *readability.Document

	doc, err = readability.NewDocument(string(content))
	if err != nil {
		return "", "", err
	}

	return doc.Content(), doc.Title, nil
}

func toMobi(content string, title string, author string, outPath string) (string, error) {
	outPath = outPath + "/" + title + ".mobi"
	m, err := mobi.NewWriter(outPath)
	if err != nil {
		return "", err
	}

	m.Title(title)
	m.Compression(mobi.CompressionNone) // LZ77 compression is also possible using  mobi.CompressionPalmDoc
	m.NewExthRecord(mobi.EXTH_DOCTYPE, "PBOK")
	m.NewExthRecord(mobi.EXTH_AUTHOR, author)
	m.NewChapter(title, []byte(content))
	// Output MOBI File
	m.Write()
	return outPath, nil
}

func PrintRss(doc *rss.Feed) {
	fmt.Printf("URL: %s\n", doc.UpdateURL)
	fmt.Printf("Title: %s\n", doc.Title)

	for key, item := range doc.Items {
		fmt.Printf(" Feed[%d]: %s\n", key, item.Title)
		fmt.Printf(" Feed[%d]:URL: %s\n", key, item.Link)
		fmt.Printf(" Feed[%d]:Published: %s\n", key, item.Date.Format(time.RFC822))
		fmt.Printf(" Feed[%d]:Summary: %s\n", key, item.Summary)

		fmt.Println()
	}
}

func createTables(c *sqlite3.Conn) (bool, error) {
	feeds := "CREATE TABLE feeds (id INTEGER PRIMARY KEY ASC, url TEXT, refresh DATETIME, last_loaded DATETIME)"
	c.Exec(feeds)

	items := "CREATE TABLE items (id INTEGER PRIMARY KEY ASC, url TEXT, feed_id INTEGER, guid TEXT, published_date DATETIME, last_loaded DATETIME, last_status INTEGER, FOREIGN KEY(feed_id) REFERENCES feeds(id))"
	c.Exec(items)

	contents := "CREATE TABLE items_contents (id INTEGER PRIMARY KEY ASC, url TEXT, item_id INTEGER, feed_id INTEGER, html_path TEXT, mobi_path TEXT, content TEXT, FOREIGN KEY(item_id) REFERENCES items(id), FOREIGN KEY(feed_id) REFERENCES feeds(id))"
	c.Exec(contents)

	parahumans := sqlite3.NamedArgs{"$feed": "https://www.parahumans.net/feed/", "$date": nil}
	insert := "INSERT INTO feeds (url, last_loaded) VALUES($feed, $date)"
	c.Exec(insert, parahumans)

	return true, nil
}

func checkFeed(url string, feed_id int64, c *sqlite3.Conn) (bool, error) {
	doc, err := rss.Fetch(url)
	if err != nil {
		return false, err
	}

	count := 0

	itemSel := "SELECT id FROM items where url = $url"
	itemIns := "INSERT INTO items (url, feed_id, guid, last_loaded) VALUES($url, $feed_id, $guid, $last_loaded)"
	contentIns := "INSERT INTO items_contents (url, item_id, feed_id, html_path, mobi_path, content) VALUES($url, $item_id, $feed_id, $html_path, $mobi_path, $content)"
	for _, item := range doc.Items {
		selItem := sqlite3.NamedArgs{"$url": item.Link}
		var itemId int64

		s, err := c.Query(itemSel, selItem)
		if err == nil {
			err := s.Scan(&itemId)
			if err == nil && itemId > 0 {
				fmt.Printf("Skipping[%d] %s\n", itemId, item.Link)
				continue
			}
		}

		res, err := http.Get(item.Link)

		if err != nil {
			return false, err
		}
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false, err
		}
		res.Body.Close()

		itemArgs := sqlite3.NamedArgs{
			"$url":          item.Link,
			"$feed_id":      feed_id,
			"$guid":         item.ID,
			"$published_at": item.Date,
			"$last_loaded":  time.Now(),
		}
		content, title, err := toReadableHtml(data)
		itemArgs["$content"] = content

		c.Exec(itemIns, itemArgs)

		itemArgs["$item_id"] = c.LastInsertId()
		if path, err := toMobi(content, title, "wildblow", "./"); err == nil {
			itemArgs["$mobi_path"] = path
			count++
		}
		c.Exec(contentIns, itemArgs)
	}

	if count == 0 {
		fmt.Printf("No new articles\n")
	} else {
		fmt.Printf("%d new articles\n", count)
	}

	return true, nil
}

func main() {
	dbPath := "feeds.db"
	var c *sqlite3.Conn
	var err error

	c, err = sqlite3.Open(dbPath)
	/*
		createTables(c)
		return
	*/

	sql := "SELECT * FROM feeds"
	var s *sqlite3.Stmt

	for s, err = c.Query(sql); err == nil; err = s.Next() {
		var feed_id int64
		var url string
		var last_loaded time.Time

		s.Scan(&feed_id, &url, &last_loaded)
		fmt.Printf("Feed %s\n", url)
		if !last_loaded.IsZero() {
			fmt.Printf("Last checked %s\n", last_loaded.Format(time.RFC3339))
		}
		if time.Now().Sub(last_loaded).Seconds() < 43200.0 {
			fmt.Println("Skipping")
			continue
		}

		checkFeed(url, feed_id, c)

		args := sqlite3.NamedArgs{"$now": time.Now()}
		updateFeed := "UPDATE feeds SET last_loaded = $now"
		c.Exec(updateFeed, args)
	}
	if err != nil {
	}

}
