package feeds

type Target struct {
	Service     TargetService
	Destination TargetDestination
}

func (k MyKindleDestination) Type() string {
	return "myk"
}

var ValidTargets = map[string]TargetService{
	"myk": ServiceMyKindle{},
	"pocket": ServicePocket{},
	//"reMarkable": "Syncs to your reMarkable cloud account",
}

type TargetDestination interface {
	Type() string
}

type TargetService interface {
	Label() string
	Description() string
}

type ServiceMyKindle struct {
	SendCredentials SMTPCreds
}

func (k ServiceMyKindle) Label() string {
	return "myKindle"
}
func (k ServiceMyKindle) Description() string {
	return "Syncs to your Kindle device through your Amazon Kindle email address"
}

type ServiceReMarkable struct {}

func (r ServiceReMarkable) Label() string {
	return "reMarkable"
}

func (r ServiceReMarkable) Description() string {
	return "to be added"
}
