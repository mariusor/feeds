package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dghubble/sessions"
	"github.com/mariusor/feeds"
)

var errorTpl = template.Must(template.New("error.html").ParseFiles("web/templates/error.html"))

var notFoundHandler = func(e error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusNotFound)
		msg := []byte(e.Error())
		msg[0] = strings.ToUpper(string(msg[0]))[0]
		errorTpl.Execute(w, string(msg))
	}
}

var validFileTypes = [...]string {
	"html",
	"mobi",
	"epub",
}

func genRoutes(dbDsn string, ss *sessions.CookieStore) *http.ServeMux {
	r := http.NewServeMux()

	c, err := feeds.DB(dbDsn)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	allFeeds, err := feeds.GetFeeds(c)
	if err != nil {
		log.Printf("unable to load feeds: %s", err)
	}

	feedsListing := feedsListing{
		Feeds: allFeeds,
	}
	for _, f := range allFeeds {
		items, err := feeds.GetItemsByFeedAndType(c, f, "html")
		if err != nil {
			panic(err)
		}
		a := articleListing{
			Feed:  f,
			Items: items,
		}
		feedPath := "/" + feeds.Slug(f.Title)
		r.HandleFunc(feedPath+"/", a.Handler)
		for _, it := range items {
			article := article{Feed: f, Item: it}
			for _, typ := range validFileTypes {
				handlerFn := notFoundHandler(fmt.Errorf("%q not found", it.Title))
				if cont, ok := it.Content[typ]; ok && fileExists(cont.Path) {
					handlerFn = article.Handler
				}
				r.HandleFunc(fmt.Sprintf("%s.%s", path.Join(feedPath, it.PathSlug()), typ), handlerFn)
			}
		}
	}
	r.HandleFunc("/", feedsListing.Handler)
	r.HandleFunc("/register/", (targets{Targets: feeds.ValidTargets}).Handler)
	for typ := range feeds.ValidTargets {
		service := feeds.Slug(typ)
		switch service {
		case "mykindle":
			r.HandleFunc(path.Join("/register", service), myKindleTarget(dbDsn, ss).Handler)
		case "pocket":
			var handlerFn http.HandlerFunc
			curPath := path.Join("/register", service)
			if p, err := feeds.PocketInit(); err != nil {
				handlerFn = notFoundHandler(fmt.Errorf("Pocket is not available: %w", err))
			} else {
				handlerFn = pocketTarget(dbDsn, curPath,ss, p).Handler
			}
			r.HandleFunc(curPath, handlerFn)
		}

	}
	return r
}

func myKindleTarget(dbPath string, ss sessions.Store) target {
	return target{
		ss: ss,
		dbPath: dbPath,
		Target: "kindle",
		Details: feeds.ServiceMyKindle{
			SendCredentials: feeds.DefaultMyKindleSender,
		},
	}
}

func pocketTarget(dbPath, curPath string, ss sessions.Store, p *feeds.ServicePocket) target {
	if p.AppName == "" {
		p.AppName = "FeedSync"
	}
	return target{
		ss: ss,
		URL:     fmt.Sprintf("http://localhost:3000%s", curPath),
		Target:  "pocket",
		dbPath:  dbPath,
		Details: p,
	}
}

type target struct {
	Target  string
	URL     string
	Details feeds.TargetService
	User    *feeds.User
	dbPath  string
	ss      sessions.Store
}

func (t target) Handler(w http.ResponseWriter, r *http.Request) {
	c, err := feeds.DB(t.dbPath)
	if err != nil {
		errorTpl.Execute(w, fmt.Errorf("invalid Pocket authorization data"))
		return
	}
	s := initSession(t.ss, r)
	defer c.Close()
	if t.Target == "pocket" {
		pD := getPocketSession(s)
		p, ok := t.Details.(*feeds.ServicePocket)
		if !ok {
			errorTpl.Execute(w, fmt.Errorf("invalid Pocket authorization data"))
			return
		}
		if pD.Target == nil {
			pD.Target = p
		}
		if pD.RequestToken == nil {
			var err error
			if p.AuthorizeURL, pD.RequestToken, err = p.GenerateAuthorizationURL(t.URL); err != nil {
				errorTpl.Execute(w, err)
				return
			}
		}
		if authTok, err := p.ObtainAccessToken(pD.RequestToken); err == nil {
			if authTok == nil {
				errorTpl.Execute(w, fmt.Errorf("invalid Pocket authorization data"))
				return
			}
			pD.Username = authTok.Username
			pD.AccessToken = authTok.AccessToken
		}
		t.User, _ = feeds.LoadUserByService(c, pD.Username, t.Target)
		if t.User == nil {
			t.User = &feeds.User{
				Services: feeds.Services{},
			}
		}
		t.User.Services["pocket"] = feeds.PocketDestination{
			AccessToken: pD.AccessToken,
			Username:    pD.Username,
		}
	feeds.SaveUserService(c, *t.User)
	s.Values["pocket"] = pD
	}
	tt, err := tpl(fmt.Sprintf("%s.html", t.Target), r)
	if err != nil {
		errorTpl.Execute(w, err)
		return
	}
	if err = s.Save(w); err != nil {
		errorTpl.Execute(w, fmt.Errorf("unable to save session: %w", err))
		return
	}
	tt.Execute(w, t)
}

func main() {
	var basePath string
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.Parse()

	basePath = path.Clean(basePath)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, 0755)
	}

	keys := [][]byte{
		{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16},
		{0x2, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x11},
	}
	ss := sessions.NewCookieStore(keys...)
	ss.Config.Domain = "localhost"
	r := genRoutes(basePath, ss)

	ticker := time.NewTicker(30 * time.Second)
	quit := make(chan struct{})
	go func(s *sessions.CookieStore) {
		for {
			select {
			case <-ticker.C:
				r = genRoutes(basePath, s)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(ss)

	listenDomain := "127.0.0.1"
	log.Fatal(http.ListenAndServe(listenDomain+":3000", r))
}

type feedsListing struct {
	Feeds []feeds.Feed
}

func (i feedsListing) Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		notFoundHandler(fmt.Errorf("feed %q not found", path.Base(r.URL.Path)))(w, r)
		return
	}
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
		"sluggify": func(s string) template.HTMLAttr {
			return template.HTMLAttr(feeds.Slug(s))
		},
		"request": func() http.Request { return *r },
		"hasHtml": has("html"),
		"hasMobi": has("mobi"),
		"hasEPub": has("epub"),
	}
}

func has(typ string) func(i feeds.Item) bool {
	return func(i feeds.Item) bool {
		p, ok := i.Content[typ]
		return ok && fileExists(p.Path)
	}
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func tpl(n string, r *http.Request) (*template.Template, error) {
	return template.New(n).Funcs(tplFuncs(r)).ParseFiles(path.Join("web/templates/", n))
}

func (a articleListing) Handler(w http.ResponseWriter, r *http.Request) {
	if path.Base(r.URL.Path) != feeds.Slug(a.Feed.Title) {
		notFoundHandler(fmt.Errorf("feed %q does not contain an article named %q", a.Feed.Title, path.Base(r.URL.Path)))(w, r)
		return
	}
	t, err := tpl("items.html", r)
	if err != nil {
		errorTpl.Execute(w, err)
		return
	}
	t.Execute(w, a)
}

type articleListing struct {
	Feed  feeds.Feed
	Items []feeds.Item
}

func (a article) Handler(w http.ResponseWriter, r *http.Request) {
	ext := strings.TrimLeft(path.Ext(r.URL.Path), ".")
	cont, ok := a.Item.Content[ext]
	if !ok {
		notFoundHandler(fmt.Errorf("%s was not found", path.Base(r.URL.Path)))
		return
	}
	http.ServeFile(w, r, cont.Path)
}

type article struct {
	Feed feeds.Feed
	Item feeds.Item
}

type targets struct {
	Targets map[string]string
}

func (t targets) Handler(w http.ResponseWriter, r *http.Request) {
	tt, err := tpl("register.html", r)
	if err != nil {
		errorTpl.Execute(w, err)
		return
	}
	tt.Execute(w, t)
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

var sessionName = "_s"
func initSession(ss sessions.Store, r *http.Request) *sessions.Session {
	gob.Register(feeds.PocketDestination{})
	s, err := ss.Get(r, sessionName)
	if err != nil {
		s = sessions.NewSession(ss, sessionName)
		s.Config = ss.(*sessions.CookieStore).Config
	}
	return s
}

func getPocketSession(s *sessions.Session) feeds.PocketDestination {
	if pocket, ok := s.Values["pocket"]; ok {
		if p, ok := pocket.(feeds.PocketDestination); ok {
			return p
		}
	}
	return feeds.PocketDestination{}
}
