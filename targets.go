package feeds

type Target struct {
	Service     DestinationService
	Destination DestinationTarget
}

var ValidTargets = map[string]DestinationService{
	"myk": ServiceMyKindle{},
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

type ServiceReMarkable struct {}

func (r ServiceReMarkable) Label() string {
	return "reMarkable"
}

func (r ServiceReMarkable) Description() string {
	return "Syncs to your reMarkable cloud account"
}

func (r ServiceReMarkable) ValidContentTypes() []string {
	return []string{"epub"/*, "pdf"*/}
}
