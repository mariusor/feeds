package feeds

import (
	"encoding/json"
	"log"
	"path"

	"github.com/motemen/go-pocket/api"
	"github.com/motemen/go-pocket/auth"
)

func NewPocket(d DestinationService) PocketDestination {
	p := PocketDestination{}
	if d != nil {
		p.Target = d.(ServicePocket)
	}
	return p
}

// PocketDestination represents the combination of a Pocket target service
// with the Pocket user authorization required to make API calls for saving items.
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
