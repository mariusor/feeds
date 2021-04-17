package feeds

import "github.com/motemen/go-pocket/auth"

type Target struct {
	Service     TargetService
	Destination TargetDestination
}

type PocketDestination struct {
	Service      *ServicePocket
	Step         int
	RequestToken *auth.RequestToken `json:"-"`
	AuthorizeURL string
	AccessToken  string `json:"access_token"`
	Username     string `json:"username"`
}

var ValidTargets = map[string]string{
	"myKindle": "Syncs to your Kindle device through your Amazon Kindle email address",
	"Pocket": "Syncs to your Pocket account",
	//"reMarkable": "Syncs to your reMarkable cloud account",
}

type TargetDestination interface { }

type TargetService interface {
	Label() string
}

type ServiceMyKindle struct {
	SendCredentials SMTPCreds
}

func (k ServiceMyKindle) Label() string {
	return "myKindle"
}

type ServiceReMarkable struct {}

func (r ServiceReMarkable) Label() string {
	return "reMarkable"
}
