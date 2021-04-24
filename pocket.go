package feeds

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"github.com/motemen/go-pocket/api"
	"github.com/motemen/go-pocket/auth"
)

var PocketConsumerKey = ""

type ServicePocket struct {
	AppName     string `json:"app_name"`
	ConsumerKey string `json:"consumer_key"`
}

func (p ServicePocket) Label() string {
	return "Pocket"
}

func (p ServicePocket) Description() string {
	return "Syncs to your Pocket account"
}

func (p ServicePocket) ValidContentTypes() []string {
	return []string{"raw"}
}

func PocketInit() (*ServicePocket, error) {
	if PocketConsumerKey == "" {
		return nil, fmt.Errorf("no Pocket application key has been set up")
	}
	return &ServicePocket{ConsumerKey: PocketConsumerKey}, nil
}

func NewPocket(d DestinationService) PocketDestination {
	p := PocketDestination{ }
	if d != nil {
		p.Target = d.(ServicePocket)
	}
	return p
}

type PocketDestination struct {
	Target       ServicePocket      `json:"target"`
	Step         int                `json:"-"`
	RequestToken *auth.RequestToken `json:"-"`
	AuthorizeURL string             `json:"-"`
	AccessToken  string             `json:"access_token"`
	Username     string             `json:"username"`
}

func (p PocketDestination) Type() string {
	return "pocket"
}

func (p PocketDestination) Service() DestinationService {
	return p.Target
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

	opt := new(api.AddOption)
	opt.URL = disp.Item.URL.String()
	opt.Title = disp.Item.Title
	opt.Tags = Slug(disp.Item.Feed.Title)

	log.Printf("Sending %s %s to %s %s", cont.Type, path.Base(cont.Path), target.Username, target.Type())
	client := api.NewClient(target.Target.ConsumerKey, target.AccessToken)
	if err := client.Add(opt); err != nil {
		return false, err
	}

	return true, nil
}
