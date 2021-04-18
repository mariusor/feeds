package feeds

type Target struct {
	Service     TargetService
	Destination TargetDestination
}

var ValidTargets = map[string]TargetService{
	"myk": ServiceMyKindle{},
	"pocket": ServicePocket{},
	//"reMarkable": "Syncs to your reMarkable cloud account",
}

type TargetDestination interface {
	Type() string
	Target() TargetService
}

type TargetService interface {
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
	return "to be added"
}

func (r ServiceReMarkable) ValidContentTypes() []string {
	return []string{"epub"/*, "pdf"*/}
}
