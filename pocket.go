package feeds

import (
	"fmt"
	"github.com/motemen/go-pocket/auth"
)

var PocketConsumerKey = ""

type ServicePocket struct {
	AppName       string `json:"app_name"`
	ConsumerKey   string `json:"consumer_key"`
}

func(p ServicePocket) Label()string {
	return "Pocket"
}

func (p ServicePocket) Description() string {
	return "Syncs to your Pocket account"
}

func PocketInit() (*ServicePocket, error) {
	if PocketConsumerKey == "" {
		return nil, fmt.Errorf("no Pocket application key has been set up")
	}
	return &ServicePocket{ConsumerKey: PocketConsumerKey}, nil
}

type PocketDestination struct {
	Service      ServicePocket      `json:"service"`
	Step         int                `json:"-"`
	RequestToken *auth.RequestToken `json:"-"`
	AuthorizeURL string             `json:"-"`
	AccessToken  string             `json:"access_token"`
	Username     string             `json:"username"`
}

func (p PocketDestination) Type() string {
	return "pocket"
}
