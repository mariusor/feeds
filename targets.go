package feeds

type Target struct {
	ID int
	Type string
	Data []byte
	Flags int
}

var ValidTargets = map[string]string{
	"myKindle": "Syncs to your Kindle device through your Amazon Kindle email address",
	"Pocket": "Syncs to your Pocket account",
	//"reMarkable": "Syncs to your reMarkable cloud account",
}

type TargetService interface {
	Label() string
}

type ServicePocket struct {
	ConsumerKey string
}

func (p ServicePocket) Label() string {
	return "Pocket"
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
