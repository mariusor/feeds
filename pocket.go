package feeds

import (
	"fmt"
	"github.com/motemen/go-pocket/auth"
)

var PocketConsumerKey = ""

type ServicePocket struct {
	AppName       string
	AuthorizeURL  string
	ConsumerKey   string
}

func(p *ServicePocket) Label()string {
	return "Pocket"
}

func PocketInit() (*ServicePocket, error) {
	if PocketConsumerKey == "" {
		return nil, fmt.Errorf("no Pocket application key has been set up")
	}
	return &ServicePocket{ConsumerKey: PocketConsumerKey}, nil
}

func (p *ServicePocket) GenerateAuthorizationURL(redirectURL string) (string, *auth.RequestToken, error) {
	requestToken, err := auth.ObtainRequestToken(p.ConsumerKey, redirectURL)
	if err != nil {
		return "", nil, err
	}

	p.AuthorizeURL = auth.GenerateAuthorizationURL(requestToken, redirectURL)
	return p.AuthorizeURL,requestToken, nil
}

func (p *ServicePocket) ObtainAccessToken(reqToken *auth.RequestToken) (*auth.Authorization, error) {
	if reqToken == nil {
		return nil, fmt.Errorf("request has not been authorized by user")
	}
	return auth.ObtainAccessToken(p.ConsumerKey, reqToken)
}
