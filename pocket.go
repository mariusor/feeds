package feeds

import (
	"fmt"
	"github.com/motemen/go-pocket/auth"
	"net/http"
	"net/http/httptest"
)

var Pocket = Target{
	ID:    2,
	Type:  "pocket",
	Flags: 0,
}

var PocketAppAccessKey = ""

func PocketInit () (*auth.Authorization, error){
	return obtainAccessToken(PocketAppAccessKey)
}

func obtainAccessToken(consumerKey string) (*auth.Authorization, error) {
	ch := make(chan struct{})
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/favicon.ico" {
				http.Error(w, "Not Found", 404)
				return
			}

			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintln(w, "Authorized.")
			ch <- struct{}{}
		}))
	defer ts.Close()

	redirectURL := ts.URL

	requestToken, err := auth.ObtainRequestToken(consumerKey, redirectURL)
	if err != nil {
		return nil, err
	}

	url := auth.GenerateAuthorizationURL(requestToken, redirectURL)
	fmt.Println(url)

	<-ch

	return auth.ObtainAccessToken(consumerKey, requestToken)
}