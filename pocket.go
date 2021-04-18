package feeds

import (
	"encoding/json"
	"fmt"
	"github.com/motemen/go-pocket/auth"
	"log"
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

func (p ServicePocket) ValidContentTypes() []string {
	return []string{"epub", "raw", "html"}
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

func (p PocketDestination) Target() TargetService {
	return p.Service
}

func DispatchToPocket(disp DispatchItem) (bool, error) {
	var target PocketDestination
	if err := json.Unmarshal(disp.Destination.Credentials, &target); err != nil {
		return false, err
	}
	var cont Content
	for _, typ := range ValidTargets[target.Type()].ValidContentTypes() {
		var ok bool
		if cont, ok = disp.Item.Content[typ]; ok {
			break
		}
	}

	log.Printf("Sending %s %s to %s %s", cont.Type, cont.Path, target.Username, target.Type())

	return true, nil
}
