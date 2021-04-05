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
