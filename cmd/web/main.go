package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dghubble/sessions"
	"github.com/mariusor/feeds"
	"github.com/motemen/go-pocket/auth"
)

var errorTpl = template.Must(template.New("error.html").ParseFiles("web/templates/error.html"))

type renderer struct {
	name string
	s    sessions.Store
}

func R(name string, s sessions.Store) renderer {
	return renderer{
		name: fmt.Sprintf("%s.html", name),
		s:    s,
	}
}

func (rr renderer) SessionInit(r *http.Request) *sessions.Session {
	return initSession(rr.s, r)
}

func (rr renderer) Redirect(w http.ResponseWriter, r *http.Request, s *sessions.Session, url string) {
	if s != nil {
		s.Save(w)
	}
	http.Redirect(w, r, url, http.StatusPermanentRedirect)
}

func (rr renderer) Write(w http.ResponseWriter, r *http.Request, s *sessions.Session, t interface{}) {
	paths := []string{
		path.Join("web/templates/", rr.name),
		"web/templates/partials/services.html",
		"web/templates/partials/new-feed.html",
		"web/templates/partials/subscriptions.html",
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

var validFileTypes = [...]string{
	"html",
	"mobi",
	"epub",
}

func genRoutes(db *sql.DB, ss sessions.Store) *http.ServeMux {
	r := http.NewServeMux()

	allFeeds, err := feeds.GetFeeds(db)
	if err != nil {
		log.Printf("unable to load feeds: %s", err)
	}

	feedsListing := index{Feeds: allFeeds, s: ss}

	r.HandleFunc("/", feedsListing.Handler)
	r.HandleFunc("/add", AddHandler(db))
	for _, f := range allFeeds {
		items, err := feeds.GetItemsByFeedAndType(db, f, "html")
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
			r.HandleFunc(path.Join("/register", service), myKindleTarget(db, ss, feedsListing.Feeds).Handler)
		case "pocket":
			var handlerFn http.HandlerFunc
			curPath := path.Join("/register", service)
			if p, err := feeds.PocketInit(); err != nil {
				handlerFn = notFoundHandler(fmt.Errorf("Pocket is not available: %w", err))
			} else {
				handlerFn = pocketTarget(db, curPath, ss, *p, feedsListing.Feeds).Handler
			}
			r.HandleFunc(curPath, handlerFn)
		}
	}
	r.HandleFunc("/subscriptions", genericTarget(db, ss, feedsListing.Feeds).HandleSubscriptions)
	return r
}

var defaultKindleService = feeds.ServiceMyKindle{
	SendCredentials: feeds.DefaultMyKindleSender,
}

func myKindleTarget(c *sql.DB, ss sessions.Store, f []feeds.Feed) target {
	t := target{
		r:           R("myk", ss),
		Feeds:       f,
		Service:     make(map[string]feeds.DestinationService),
		Destination: make(map[string]feeds.DestinationTarget),
		db:          c,
	}
	t.Service["myk"] = &defaultKindleService
	return t
}

func genericTarget(c *sql.DB, ss sessions.Store, f []feeds.Feed) target {
	t := target{
		r:           R("subs", ss),
		Feeds:       f,
		db:          c,
		Service:     make(map[string]feeds.DestinationService),
		Destination: make(map[string]feeds.DestinationTarget),
	}
	t.Service["pocket"] = &defaultPocketService
	t.Service["myk"] = &defaultKindleService
	return t
}

var defaultPocketService = feeds.ServicePocket{
	AppName:     feeds.PocketAppName,
	ConsumerKey: feeds.PocketConsumerKey,
}

func pocketTarget(c *sql.DB, curPath string, ss sessions.Store, p feeds.ServicePocket, f []feeds.Feed) target {
	t := target{
		URLPath:     curPath,
		r:           R("pocket", ss),
		Feeds:       f,
		db:          c,
		Service:     make(map[string]feeds.DestinationService),
		Destination: make(map[string]feeds.DestinationTarget),
	}
	t.Service["pocket"] = &defaultPocketService
	return t
}

type target struct {
	r             renderer
	URLPath       string
	Service       map[string]feeds.DestinationService
	Destination   map[string]feeds.DestinationTarget
	Feeds         []feeds.Feed
	Subscriptions []feeds.Subscription
	db            *sql.DB
}

const (
	PocketAuthStepTokenGenerated = 1 << iota
	PocketAuthStepAuthLinkGenerated
	PocketAuthStepAuthorized

	PocketAuthDisabled   = -1
	PocketAuthNotStarted = 0
)

func (t target) HandleKindle(w http.ResponseWriter, r *http.Request) {
	s := t.r.SessionInit(r)
	var service *feeds.ServiceMyKindle
	if ss, ok := t.Service["myk"]; ok {
		if sss, ok := ss.(*feeds.ServiceMyKindle); ok {
			service = sss
		}
	}
	if service == nil {
		errorTpl.Execute(w, fmt.Errorf("invalid service"))
		return
	}
	kindle := getKindleSession(s, service)
	if r.Method == http.MethodPost {
		email := r.FormValue("myk_account")
		if !strings.Contains(email, "@kindle.com") {
			errorTpl.Execute(w, fmt.Errorf("please use a valid Kindle email address"))
			return
		}
		kindle.To = email
		if _, err := feeds.SaveDestination(t.db, kindle); err != nil {
			errorTpl.Execute(w, err)
			return
		}
		t.r.Redirect(w, r, s, "/subscriptions")
	}
	s.Values["kindle"] = kindle
	t.Service["myk"] = service
	t.Destination["myk"] = kindle
	t.r.Write(w, r, s, t)
}

func (t target) HandlePocketTarget(w http.ResponseWriter, r *http.Request) {
	s := t.r.SessionInit(r)
	var service *feeds.ServicePocket
	if ss, ok := t.Service["pocket"]; ok {
		if sss, ok := ss.(*feeds.ServicePocket); ok {
			service = sss
		}
	}
	if service == nil {
		errorTpl.Execute(w, fmt.Errorf("invalid service"))
		return
	}

	redirUrl := reqURL(r)
	pocket := getPocketSession(s, service)

	switch pocket.Step {
	case PocketAuthDisabled:
		errorTpl.Execute(w, fmt.Errorf("Pocket service it out of order"))
		return
	case PocketAuthNotStarted:
		if pocket.RequestToken == nil {
			requestToken, err := auth.ObtainRequestToken(service.ConsumerKey, redirUrl)
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
			pocket.AuthorizeURL = auth.GenerateAuthorizationURL(pocket.RequestToken, redirUrl)
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
				if _, err := feeds.SaveDestination(t.db, pocket); err != nil {
					errorTpl.Execute(w, err)
					return
				}
			}
		}
		s.Values["pocket"] = pocket
		t.Destination["pocket"] = pocket
		t.Service["pocket"] = service
		t.r.Redirect(w, r, s, reqURL(r))
	case PocketAuthStepAuthorized:
		s.Values["pocket"] = pocket
		t.Destination["pocket"] = pocket
		t.Service["pocket"] = service
		t.r.Redirect(w, r, s, "/subscriptions")
		return
	}
	s.Values["pocket"] = pocket
	t.Destination["pocket"] = pocket
	t.r.Write(w, r, s, t)
}

func (t target) HandleSubscriptions(w http.ResponseWriter, r *http.Request) {
	s := t.r.SessionInit(r)
	var (
		dest            *feeds.Destination
		err             error
		haveDestination bool
	)

	if d, ok := s.Values["pocket"]; ok {
		t.Destination["pocket"], ok = d.(feeds.PocketDestination)
		haveDestination = haveDestination || ok
	}
	if d, ok := s.Values["kindle"]; ok {
		t.Destination["myk"], ok = d.(feeds.MyKindleDestination)
		haveDestination = haveDestination || ok
	}
	if !haveDestination {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		r.ParseForm()
		feedIds := make([]int, 0)
		removeIds := make([]int, 0)
		if subs, ok := r.Form["sub"]; ok {
			for _, sub := range subs {
				if id, err := strconv.ParseInt(sub, 0, 0); err == nil {
					feedIds = append(feedIds, int(id))
				}
			}
		}
		ff := make([]feeds.Feed, 0)
		for _, feed := range t.Feeds {
			remove := true
			for _, id := range feedIds {
				if id == feed.ID {
					ff = append(ff, feed)
					remove = false
				}
			}
			if remove {
				removeIds = append(removeIds, feed.ID)
			}
		}

		if d, ok := t.Destination["pocket"]; ok {
			if dest, err = feeds.LoadDestination(t.db, d); err == nil {
				serv := feeds.ServicePocket{}
				json.Unmarshal(dest.Credentials, &serv)
				t.Service["pocket"] = &serv
				if err = feeds.RemoveSubscriptions(t.db, *dest, removeIds...); err != nil {
					errorTpl.Execute(w, err)
					return
				}
				if err = feeds.SaveSubscriptions(t.db, *dest, ff...); err != nil {
					errorTpl.Execute(w, err)
					return
				}
			}
		}

		if d, ok := t.Destination["myk"]; ok {
			if dest, err = feeds.LoadDestination(t.db, d); err == nil {
				serv := feeds.ServiceMyKindle{}
				json.Unmarshal(dest.Credentials, &serv)
				t.Service["myk"] = &serv
				if err = feeds.RemoveSubscriptions(t.db, *dest, removeIds...); err != nil {
					errorTpl.Execute(w, err)
					return
				}
				if err = feeds.SaveSubscriptions(t.db, *dest, ff...); err != nil {
					errorTpl.Execute(w, err)
					return
				}
			}
		}
		t.r.Redirect(w, r, s, reqURL(r))
		return
	}
	for _, d := range t.Destination {
		dest, _ = feeds.LoadDestination(t.db, d)
	}
	if dest == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if t.Subscriptions, err = feeds.LoadSubscriptions(t.db, *dest); err != nil {
		errorTpl.Execute(w, err)
		return
	}
	t.r.Write(w, r, s, t)
}

func reqURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)
}

func (t target) Handler(w http.ResponseWriter, r *http.Request) {
	which := path.Base(r.URL.Path)
	if which == "pocket" {
		t.HandlePocketTarget(w, r)
		return
	}
	if which == "myk" {
		t.HandleKindle(w, r)
		return
	}
}

func getSessionKey() []byte {
	r := rand.New(rand.NewSource(time.Now().UnixMicro()))
	b := make([]byte, 16)
	binary.LittleEndian.PutUint64(b, r.Uint64())
	binary.LittleEndian.PutUint64(b[8:], r.Uint64())
	return b
}

func main() {
	var (
		listen, basePath string
	)
	flag.StringVar(&basePath, "path", ".cache", "Base path")
	flag.StringVar(&listen, "listen", "localhost:3000", "The HTTP address to listen on")
	flag.Parse()

	basePath = path.Clean(basePath)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	keys := [][]byte{getSessionKey()}

	ss := sessions.NewCookieStore(keys...)
	ss.Config.Domain = "localhost"
	r := genRoutes(c, ss)

	ticker := time.NewTicker(30 * time.Second)
	quit := make(chan struct{})
	go func(s *sessions.CookieStore) {
		for {
			select {
			case <-ticker.C:
				r = genRoutes(c, s)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(ss)

	log.Fatal(http.ListenAndServe(listen, r))
}

type index struct {
	s     sessions.Store
	Feeds []feeds.Feed
}

type feedListing struct {
	Feeds        []feeds.Feed
	Destinations []feeds.DestinationTarget
	Targets      map[string]feeds.DestinationService
}

func (i index) Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		notFoundHandler(fmt.Errorf("feed %q not found", path.Base(r.URL.Path)))(w, r)
		return
	}

	l := feedListing{
		Feeds:        i.Feeds,
		Destinations: make([]feeds.DestinationTarget, 0),
		Targets:      feeds.ValidTargets,
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
		"request":             func() http.Request { return *r },
		"hasHtml":             has("html"),
		"hasMobi":             has("mobi"),
		"hasEPub":             has("epub"),
		"serviceEnabled":      serviceEnabled,
		"subscriptionEnabled": subscriptionEnabled,
	}
}

func subscriptionEnabled(feedId int, subscriptions []feeds.Subscription) bool {
	for _, sub := range subscriptions {
		if sub.Feed.ID == feedId {
			return true
		}
	}
	return false
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
	s            sessions.Store
	Targets      map[string]feeds.DestinationService
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
	return feeds.PocketDestination{}
}

func getKindleSession(s *sessions.Session, d feeds.DestinationService) feeds.MyKindleDestination {
	if pocket, ok := s.Values["kindle"]; ok {
		if p, ok := pocket.(feeds.MyKindleDestination); ok {
			return p
		}
	}
	return feeds.MyKindleDestination{}
}

type AddStatus struct {
	Status string
	URL    string
}

func AddHandler(db *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		feedUrl := r.FormValue("feed-url")
		if len(feedUrl) == 0 {
			errorTpl.Execute(w, fmt.Errorf("empty URL"))
			return
		}
		if r.Method == http.MethodPost {
			u, err := url.ParseRequestURI(feedUrl)
			if err != nil {
				errorTpl.Execute(w, fmt.Errorf("invalid URL %w", err))
				return
			}
			doc, err := feeds.GetFeedInfo(*u)
			if err != nil {
				errorTpl.Execute(w, fmt.Errorf("invalid RSS %w", err))
				return
			}
			feed := feeds.Feed{
				URL:       u,
				Title:     doc.Title,
				Author:    "Unknown",
				Frequency: time.Hour * 24 * 2,
			}

			if err := feeds.SaveFeeds(db, feed); err != nil {
				errorTpl.Execute(w, fmt.Errorf("invalid URL %w", err))
				return
			}
			redirect := *r.URL
			if !redirect.Query().Has("feed-url") {
				q := redirect.Query()
				q.Add("feed-url", feedUrl)
				redirect.RawQuery = q.Encode()
			}
			http.Redirect(w, r, redirect.String(), http.StatusSeeOther)
		}
		var a = AddStatus{
			Status: "OK",
			URL:    feedUrl,
		}
		t, err := tpl("add.html", r)
		if err != nil {
			errorTpl.Execute(w, err)
			return
		}
		t.Execute(w, a)
	}
}
