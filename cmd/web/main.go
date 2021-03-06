package main

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dghubble/sessions"
	"github.com/mariusor/feeds"
	"github.com/motemen/go-pocket/auth"
)

var errorTpl = template.Must(template.New("error.html").ParseFiles("web/templates/error.html"))

type renderer struct {
	name string
	s   sessions.Store
}

func R(name string, s sessions.Store) renderer {
	return renderer{
		name: fmt.Sprintf("%s.html", name),
		s: s,
	}
}

func (rr renderer) SessionInit(r *http.Request) *sessions.Session {
	return initSession(rr.s, r)
}

func (rr renderer) Redirect(w http.ResponseWriter, r *http.Request, s *sessions.Session, url string) {
	if s != nil {
		if err := s.Save(w); err != nil {
			errorTpl.Execute(w, fmt.Errorf("unable to save session: %w", err))
			return
		}
	}
	http.Redirect(w, r, url, http.StatusPermanentRedirect)
}

func (rr renderer) Write(w http.ResponseWriter, r *http.Request, s *sessions.Session, t interface{}) {
	path := path.Join("web/templates/", rr.name)
	paths := []string{
		path,
		"web/templates/partials/services.html",
		"web/templates/partials/new-feed.html",
	}
	tpl, err := template.New(rr.name).Funcs(tplFuncs(r)).ParseFiles(paths...)
	if err != nil {
		errorTpl.Execute(w, err)
		return
	}
	if s != nil {
		if err := s.Save(w); err != nil {
			errorTpl.Execute(w, fmt.Errorf("unable to save session: %w", err))
			return
		}
	}
	tplW := new(bytes.Buffer)
	if err := tpl.Execute(tplW, t); err != nil {
		errorTpl.Execute(w, err)
		return
	}
	if _, err := io.Copy(w, tplW); err != nil {
		errorTpl.Execute(w, err)
	}
}

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

func genRoutes(dbDsn string, ss sessions.Store) *http.ServeMux {
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

	feedsListing := index{Feeds: allFeeds, s: ss}

	r.HandleFunc("/", feedsListing.Handler)
	r.HandleFunc("/add", AddHandler)
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
	r.HandleFunc("/register/", (targets{Targets: feeds.ValidTargets, s: ss}).Handler)
	for typ := range feeds.ValidTargets {
		service := feeds.Slug(typ)
		switch service {
		case "myk":
			r.HandleFunc(path.Join("/register", service), myKindleTarget(dbDsn, ss, feedsListing.Feeds).Handler)
		case "pocket":
			var handlerFn http.HandlerFunc
			curPath := path.Join("/register", service)
			if p, err := feeds.PocketInit(); err != nil {
				handlerFn = notFoundHandler(fmt.Errorf("Pocket is not available: %w", err))
			} else {
				handlerFn = pocketTarget(dbDsn, curPath, ss, *p, feedsListing.Feeds).Handler
			}
			r.HandleFunc(curPath, handlerFn)
		}

	}
	return r
}

func myKindleTarget(dbPath string, ss sessions.Store, f []feeds.Feed) target {
	return target{
		dbPath: dbPath,
		r:      R("myk", ss),
		Feeds: f,
		Service: feeds.ServiceMyKindle{
			SendCredentials: feeds.DefaultMyKindleSender,
		},
	}
}

func pocketTarget(dbPath, curPath string, ss sessions.Store, p feeds.ServicePocket, f []feeds.Feed) target {
	if p.AppName == "" {
		p.AppName = "FeedSync"
	}
	return target{
		URL:     fmt.Sprintf("http://localhost:3000%s", curPath),
		r:       R("pocket", ss),
		Feeds: f,
		dbPath:  dbPath,
		Service: p,
	}
}

type target struct {
	r           renderer
	URL         string
	Service     feeds.DestinationService
	Destination feeds.DestinationTarget
	Feeds       []feeds.Feed
	dbPath      string
}

const (
	PocketAuthStepTokenGenerated = 1 << iota
	PocketAuthStepAuthLinkGenerated
	PocketAuthStepAuthorized

	PocketAuthDisabled           = -1
	PocketAuthNotStarted         = 0
)

func (t target) Handler(w http.ResponseWriter, r *http.Request) {
	c, err := feeds.DB(t.dbPath)
	if err != nil {
		errorTpl.Execute(w, fmt.Errorf("invalid Pocket authorization data"))
		return
	}
	defer c.Close()

	s := t.r.SessionInit(r)
	var dest *feeds.Destination
	if strings.ToLower(t.Service.Label()) == "pocket" {
		service, _ := t.Service.(feeds.ServicePocket)
		pocket := getPocketSession(s, service)
		switch pocket.Step {
		case PocketAuthDisabled:
			errorTpl.Execute(w, fmt.Errorf("Pocket service it out of order"))
			return
		case PocketAuthNotStarted:
			if pocket.RequestToken == nil {
				var err error
				requestToken, err := auth.ObtainRequestToken(service.ConsumerKey, t.URL)
				if err != nil {
					errorTpl.Execute(w, fmt.Errorf("invalid Pocket authorization data"))
					return
				}
				pocket.RequestToken = requestToken
				pocket.Step = PocketAuthStepTokenGenerated
			}
			fallthrough
		case PocketAuthStepTokenGenerated:
			if pocket.AuthorizeURL == "" && pocket.RequestToken != nil {
				pocket.AuthorizeURL = auth.GenerateAuthorizationURL(pocket.RequestToken, t.URL)
			}
			if pocket.AuthorizeURL != "" {
				pocket.Step = PocketAuthStepAuthLinkGenerated
			}
		case PocketAuthStepAuthLinkGenerated:
			if pocket.AccessToken == "" && pocket.RequestToken != nil {
				if authTok, err := auth.ObtainAccessToken(service.ConsumerKey, pocket.RequestToken); err != nil {
					if strings.Contains(err.Error(), "403") {
						pocket.Step = PocketAuthNotStarted
					}
					if strings.Contains(err.Error(), "429") {
						pocket.Step = PocketAuthDisabled
					}
				} else {
					pocket.RequestToken = nil
					pocket.Username = authTok.Username
					pocket.AccessToken = authTok.AccessToken
					pocket.Step = PocketAuthStepAuthorized
					if dest, err = feeds.SaveDestination(c, pocket); err != nil {
						errorTpl.Execute(w, err)
						return
					}
				}
			}
			fallthrough
		case PocketAuthStepAuthorized:
		}

		s.Values["pocket"] = pocket
		t.Destination = pocket
	}
	if strings.ToLower(t.Service.Label()) == "mykindle" {
		service, _ := t.Service.(feeds.ServiceMyKindle)
		kindle := getKindleSession(s, service)
		if r.Method == http.MethodPost {
			email := r.FormValue("myk_account")
			if !strings.Contains(email, "@kindle.com") {
				errorTpl.Execute(w, fmt.Errorf("please use a valid Kindle email address"))
				return
			}
			kindle.To = email
			if dest, err = feeds.SaveDestination(c, kindle); err != nil {
				errorTpl.Execute(w, err)
				return
			}
		}
		s.Values["kindle"] = kindle
		t.Destination = kindle
	}
	if dest != nil {
		if err = feeds.SaveSubscriptions(c, *dest, t.Feeds...); err != nil {
			errorTpl.Execute(w, err)
		}
	}
	t.r.Write(w, r, s, t)
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

type index struct {
	s sessions.Store
	Feeds []feeds.Feed
}

type feedListing struct {
	Feeds []feeds.Feed
	Destinations []feeds.DestinationTarget
	Targets map[string]feeds.DestinationService
}

func (i index) Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		notFoundHandler(fmt.Errorf("feed %q not found", path.Base(r.URL.Path)))(w, r)
		return
	}

	l := feedListing{
		Feeds: i.Feeds,
		Destinations: make([]feeds.DestinationTarget, 0),
		Targets: feeds.ValidTargets,
	}
	rr := R("index", i.s)
	s := rr.SessionInit(r)
	pocket := getPocketSession(s, nil)
	if pocket.AccessToken != "" {
		l.Destinations = append(l.Destinations, pocket)
	}
	kindle := getKindleSession(s, nil)
	if kindle.To != "" {
		l.Destinations = append(l.Destinations, kindle)
	}
	rr.Write(w, r, s, l)
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
		"ServiceEnabled": serviceEnabled,
	}
}

func serviceEnabled(dest []feeds.DestinationTarget, typ string) bool {
	for _, d := range dest {
		if d.Type() == typ {
			return true
		}
	}
	return false
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
	s sessions.Store
	Targets map[string]feeds.DestinationService
	Destinations []feeds.DestinationTarget
}

func (t targets) Handler(w http.ResponseWriter, r *http.Request) {
	rr := R("register", t.s)
	s := rr.SessionInit(r)
	pocket := getPocketSession(s, nil)
	if pocket.AccessToken != "" {
		t.Destinations = append(t.Destinations, pocket)
	}
	kindle := getKindleSession(s, nil)
	if kindle.To != "" {
		t.Destinations = append(t.Destinations, kindle)
	}
	rr.Write(w, r, s, t)
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
	gob.Register(feeds.MyKindleDestination{})
	s, err := ss.Get(r, sessionName)
	if err != nil {
		s = sessions.NewSession(ss, sessionName)
		s.Config = ss.(*sessions.CookieStore).Config
	}
	return s
}

func getPocketSession(s *sessions.Session, d feeds.DestinationService) feeds.PocketDestination {
	if pocket, ok := s.Values["pocket"]; ok {
		if p, ok := pocket.(feeds.PocketDestination); ok {
			return p
		}
	}
	return feeds.NewPocket(d)
}

func getKindleSession(s *sessions.Session, d feeds.DestinationService) feeds.MyKindleDestination {
	if pocket, ok := s.Values["kindle"]; ok {
		if p, ok := pocket.(feeds.MyKindleDestination); ok {
			return p
		}
	}
	return feeds.NewMyKindle(d)
}

type AddStatus struct {
	Status string
	URL string
}

func AddHandler(w http.ResponseWriter, r *http.Request) {
	feedUrl := r.FormValue("feed-url")
	if len(feedUrl) == 0 {
		errorTpl.Execute(w, fmt.Errorf("empty URL"))
		return
	}
	var a = AddStatus{
		Status: "OK",
		URL: feedUrl,
	}
	t, err := tpl("add.html", r)
	if err != nil {
		errorTpl.Execute(w, err)
		return
	}
	t.Execute(w, a)
}
