package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/mariusor/feeds"
)

var errorTpl = template.Must(template.New("error.html").ParseFiles("web/templates/error.html"))

func logMw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func authMw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func main() {
	var basePath string
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.Parse()

	basePath = path.Clean(basePath)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, 0755)
	}

	htmlBasePath := path.Join(basePath, feeds.HtmlDir)
	if _, err := os.Stat(htmlBasePath); os.IsNotExist(err) {
		os.Mkdir(htmlBasePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	listenDomain := "127.0.0.1"

	r := http.NewServeMux()
	allFeeds, err := feeds.GetFeeds(c)
	if err != nil {
		log.Printf("unable to load feeds: %s", err)
	}

	feedsListing := feedsListing{
		Feeds:     allFeeds,
	}
	for _, f := range allFeeds {
		items, err := feeds.GetContentsByFeed(c, f)
		if err != nil {
			panic(err)
		}
		a := articleListing{
			Feed: f,
			Items: items,
		}
		feedPath := "/" + sluggifyTitle(f.Title) + "/"
		r.HandleFunc(feedPath, a.Handler)
		for _, c := range items {
			article := article{ Feed: f, Item: c }
			articlePath := path.Join(feedPath, sluggifyTitle(c.Item.Title)) + "/"
			r.HandleFunc(articlePath, article.Handler)
		}
	}
	r.HandleFunc("/", feedsListing.Handler)
	log.Fatal(http.ListenAndServe(listenDomain+":3000", r))
}

type feedsListing struct {
	Feeds     []feeds.Feed
}

func (i feedsListing) Handler(w http.ResponseWriter, r *http.Request) {
	t, err := tpl("index.html", r)
	if err != nil {
		errorTpl.Execute(w, err)
		return
	}
	t.Execute(w, i)
}

var tplFuncs = func(r *http.Request) template.FuncMap {
	return template.FuncMap{
		"fmtDuration": fmtDuration,
		"sluggify": func (s string) template.HTMLAttr {
			return template.HTMLAttr(sluggifyTitle(s))
		},
		"request": func () http.Request { return *r },
	}
} 

func tpl(n string, r *http.Request) (*template.Template, error) {
	return template.New(n).Funcs(tplFuncs(r)).ParseFiles(path.Join("web/templates/", n))
}

func (a articleListing) Handler (w http.ResponseWriter, r *http.Request) {
	t, err := tpl("items.html", r)
	if err != nil {
		errorTpl.Execute(w, err)
		return
	}
	t.Execute(w, a)
}

type articleListing struct {
	Feed feeds.Feed
	Items []feeds.Content
}

func (a article) Handler (w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, a.Item.HTMLPath)
}

type article struct {
	Feed feeds.Feed
	Item feeds.Content
}

func sluggifyTitle(s string) string {
	s = strings.Map(func (r rune) rune {
		switch r {
		case ',', '?', '!', '\'', '`':
			return -1
		}
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'z') {
			return r
		}
		return '-'
	}, strings.ToLower(s))
	rr := regexp.MustCompile("-+")
	b := rr.ReplaceAll([]byte(s), []byte{'-'})
	if b[len(b)-1] == '-' {
		b = b[:len(b)-1]
	}
	return string(b)
}

func fmtDuration(d time.Duration) template.HTML {
	var (
		unit          = "week"
		times float32 = -1.0
	)
	timesFn := func(d1, d2 time.Duration) float32 {
		return float32(float64(d1) / float64(d2))
	}
	if d <= 0 {
		return "never"
	}
	day := 24 * time.Hour
	week := 7 * day
	times = timesFn(week, d)
	if times > 6 {
		unit = "day"
		times = timesFn(day, d)
		if times > 20 {
			unit = "hour"
			times = timesFn(time.Hour, d)
			if times > 29 {
				unit = "minute"
				times = timesFn(time.Minute, d)
			}
		}
	}
	if times == 1 && unit != "minute" {
		if unit == "day" {
			unit = "daily"
		} else {
			unit = unit + "ly"
		}
		return template.HTML(unit)
	}
	return template.HTML(fmt.Sprintf("%.1f times per %s", times, unit))
}
