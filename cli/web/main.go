package main

import (
	"html/template"
	"net/http"
	"os"

	"sort"

	"log"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
)

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
	listenDomain := "myk.localdomain"
	goth.UseProviders(
		//twitter.New(os.Getenv("TWITTER_KEY"), os.Getenv("TWITTER_SECRET"), "http://"+listenDomain+":3000/auth/twitter/callback"),
		// If you'd like to use authenticate instead of authorize in Twitter provider, use this instead.
		//twitter.NewAuthenticate(os.Getenv("TWITTER_KEY"), os.Getenv("TWITTER_SECRET"), "http://"+listenDomain+":3000/auth/twitter/callback"),

		//facebook.New(os.Getenv("FACEBOOK_KEY"), os.Getenv("FACEBOOK_SECRET"), "http://"+listenDomain+":3000/auth/facebook/callback"),
		//gplus.New(os.Getenv("GPLUS_KEY"), os.Getenv("GPLUS_SECRET"), "http://localhost:3000/auth/gplus/callback"),
		github.New(os.Getenv("GITHUB_KEY"), os.Getenv("GITHUB_SECRET"), "http://"+listenDomain+":3000/auth/github/callback"),
		//gitlab.New(os.Getenv("GITLAB_KEY"), os.Getenv("GITLAB_SECRET"), "http://"+listenDomain+":3000/auth/gitlab/callback"),
	)

	m := make(map[string]string)
	//m["facebook"] = "Facebook"
	//m["twitter"] = "Twitter"
	m["github"] = "Github"
	//m["gitlab"] = "Gitlab"
	//m["gplus"] = "Google Plus"

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
	providerIndex := &ProviderIndex{Providers: keys, ProvidersMap: m}

	r := mux.NewRouter()
	r.Use(logMw)
	//r.Use(authMw)

	r.HandleFunc("/auth/{provider}/callback", func(res http.ResponseWriter, req *http.Request) {
		user, err := gothic.CompleteUserAuth(res, req)
		if err != nil {
			t, _ := template.New("error.html").ParseFiles("src/web/templates/error.html")
			t.Execute(res, err)
			return
		}
		t, _ := template.New("user.html").ParseFiles("src/web/templates/user.html")
		t.Execute(res, user)
	})

	r.HandleFunc("/logout/{provider}", func(res http.ResponseWriter, req *http.Request) {
		gothic.Logout(res, req)
		res.Header().Set("Location", "/")
		res.WriteHeader(http.StatusTemporaryRedirect)
	})

	r.HandleFunc("/auth/{provider}", func(res http.ResponseWriter, req *http.Request) {
		// try to get the user without re-authenticating
		if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
			t, _ := template.New("user.html").ParseFiles("src/web/templates/user.html")
			t.Execute(res, gothUser)
			return
		}
		gothic.BeginAuthHandler(res, req)
	})

	r.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		var t *template.Template
		if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
			t, _ = template.New("user.html").ParseFiles("src/web/templates/user.html")
			t.Execute(res, gothUser)
		} else {
			t,_ = template.New("index.html").ParseFiles("src/web/templates/index.html")
			t.Execute(res, providerIndex)
		}
	})

	log.Fatal(http.ListenAndServe(listenDomain + ":3000", r))
}

type ProviderIndex struct {
	Providers    []string
	ProvidersMap map[string]string
}
