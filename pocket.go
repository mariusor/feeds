package feeds

import (
	"fmt"
	"github.com/motemen/go-pocket/auth"
)

var Pocket = Target{
	ID:    2,
	Type:  "pocket",
	Flags: 0,
}

var PocketConsumerKey = ""

type PocketAuth struct {
	AppName       string
	RequestToken  *auth.RequestToken
	Authorization *auth.Authorization
	AuthorizeURL  string
	ConsumerKey   string
}

func(p *PocketAuth) Label()string {
	return "Pocket(deprecated)"
}

func PocketInit() (*PocketAuth, error) {
	if PocketConsumerKey == "" {
		return nil, fmt.Errorf("no Pocket application key has been set up")
	}
	return &PocketAuth{ConsumerKey: PocketConsumerKey}, nil
}

func (p *PocketAuth) GenerateAuthorizationURL(redirectURL string) (string, error) {
	var err error

	if p.RequestToken, err = auth.ObtainRequestToken(p.ConsumerKey, redirectURL); err != nil {
		return "", err
	}

	p.AuthorizeURL = auth.GenerateAuthorizationURL(p.RequestToken, redirectURL)
	return p.AuthorizeURL, nil
}

func (p *PocketAuth) ObtainAccessToken() error {
	var err error
	if p.RequestToken == nil {
		return fmt.Errorf("request has not been authorized by user")
	}
	p.Authorization, err = auth.ObtainAccessToken(p.ConsumerKey, p.RequestToken)
	return err
}
