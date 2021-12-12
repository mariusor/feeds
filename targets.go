package feeds

import "fmt"

var ValidTargets = map[string]DestinationService{
	"myk":    ServiceMyKindle{},
	"pocket": ServicePocket{},
	//"reMarkable": ServiceReMarkable{},
}

type DestinationTarget interface {
	Type() string
	Service() DestinationService
}

type DestinationService interface {
	Label() string
	Description() string
	ValidContentTypes() []string
}

// ServiceMyKindle is the target service for Kindle devices
// It consists of email credentials that are used to email a Kindle email address.
// The owner of the Kindle email needs to white-list this email address
type ServiceMyKindle struct {
	SendCredentials SMTPCreds `json:"send_credentials"`
}

func (k ServiceMyKindle) Label() string {
	return "myKindle"
}
func (k ServiceMyKindle) Description() string {
	return "Syncs to your Kindle device through your Amazon Kindle email address"
}

func (k ServiceMyKindle) ValidContentTypes() []string {
	return []string{"mobi"}
}

// ServiceReMarkable is the target service for reMarkable devices
// TODO(marius)
type ServiceReMarkable struct{}

func (r ServiceReMarkable) Label() string {
	return "reMarkable"
}

func (r ServiceReMarkable) Description() string {
	return "Syncs to your reMarkable cloud account"
}

func (r ServiceReMarkable) ValidContentTypes() []string {
	return []string{"epub" /*, "pdf"*/}
}

var PocketConsumerKey = ""
var PocketAppName = "FeedSync"

// ServicePocket is the target for Pocket integration
// It consists of OAuth client credentials for a Pocket developer app.
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
