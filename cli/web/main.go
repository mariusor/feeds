package main

import (
	"flag"
	"fmt"
	"github.com/mariusor/feeds"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
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
	goth.UseProviders(
		//twitter.New(os.Getenv("TWITTER_KEY"), os.Getenv("TWITTER_SECRET"), "http://"+listenDomain+":3000/auth/twitter/callback"),
		// If you'd like to use authenticate instead of authorize in Twitter provider, use this instead.
		//twitter.NewAuthenticate(os.Getenv("TWITTER_KEY"), os.Getenv("TWITTER_SECRET"), "http://"+listenDomain+":3000/auth/twitter/callback"),
		//facebook.New(os.Getenv("FACEBOOK_KEY"), os.Getenv("FACEBOOK_SECRET"), "http://"+listenDomain+":3000/auth/facebook/callback"),
		github.New(os.Getenv("GITHUB_KEY"), os.Getenv("GITHUB_SECRET"), "http://"+listenDomain+":3000/auth/github/callback"),
		//gitlab.New(os.Getenv("GITLAB_KEY"), os.Getenv("GITLAB_SECRET"), "http://"+listenDomain+":3000/auth/gitlab/callback"),
	)

	m := make(map[string]string)
	//m["facebook"] = "Facebook"
	//m["twitter"] = "Twitter"
	m["github"] = "Github"
	//m["gitlab"] = "Gitlab"

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	key := securecookie.GenerateRandomKey(32)
	maxAge := 8600 * 30
	store := sessions.NewFilesystemStore("", key)
	store.Options.Path = "/"
	store.Options.Domain = listenDomain
	store.Options.HttpOnly = true
	store.Options.MaxAge = maxAge

	gothic.Store = store

	sort.Strings(keys)

	r := mux.NewRouter()
	r.Use(logMw)
	//r.Use(authMw)
	feeds, err := feeds.GetFeeds(c)
	if err != nil {
		log.Printf("unable to load feeds: %s", err)
	}
	providerIndex := &Index{
		Providers: m,
		Feeds:     feeds,
	}

	r.HandleFunc("/auth/{provider}/callback", func(w http.ResponseWriter, req *http.Request) {
		user, err := gothic.CompleteUserAuth(w, req)
		if err != nil {
			errorTpl.Execute(w, err)
			return
		}
		t, _ := template.New("user.html").ParseFiles("web/templates/user.html")
		t.Execute(w, user)
	})

	r.HandleFunc("/logout/{provider}", func(w http.ResponseWriter, req *http.Request) {
		gothic.Logout(w, req)
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	r.HandleFunc("/auth/{provider}", func(w http.ResponseWriter, req *http.Request) {
		// try to get the user without re-authenticating
		gothUser, err := gothic.CompleteUserAuth(w, req)
		if err != nil {
			errorTpl.Execute(w, err)
			return
		}
		t, _ := template.New("user.html").ParseFiles("web/templates/user.html")
		t.Execute(w, gothUser)
		gothic.BeginAuthHandler(w, req)
	})

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		t, err := template.New("index.html").Funcs(template.FuncMap{
			"fmtDuration": fmtDuration,
		}).ParseFiles("web/templates/index.html")
		if err != nil {
			errorTpl.Execute(w, err)
			return
		}
		t.Execute(w, providerIndex)
	})
	log.Fatal(http.ListenAndServe(listenDomain+":3000", r))
}

type Index struct {
	Providers ProviderList
	Feeds     []feeds.Feed
}
type ProviderList map[string]string

func fmtDuration(d time.Duration) template.HTML {
	var (
		unit = "week"
		times float32 = -1.0
	)
	timesFn := func (d1, d2 time.Duration) float32 {
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