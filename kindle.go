package feeds

import (
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"

	"github.com/jordan-wright/email"
	_ "modernc.org/sqlite"
)

var (
	SMTPServer = "smtp.example.com"
	SMTPPort = "587"
	SMTPFrom = "feedsync@example.com"
	SMTPPassword = ""
	DefaultMyKindleSender = SMTPCreds{
		Server:   SMTPServer,
		Port:     SMTPPort,
		From:     SMTPFrom,
		User:     "FeedSync",
		Password: SMTPPassword,
	}
)

type SMTPCreds struct {
	Server   string `json:"server"`
	Port     string `json:"port"`
	From     string `json:"from"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func NewMyKindle(d DestinationService) MyKindleDestination {
	k := MyKindleDestination{}
	if d != nil {
		k.Target = d.(ServiceMyKindle)
	}
	return k
}

type MyKindleDestination struct {
	Target ServiceMyKindle `json:"target"`
	To     string          `json:"to"`
}

func (k MyKindleDestination) Type() string {
	return "myk"
}

func (k MyKindleDestination) Service() DestinationService {
	return k.Target
}

func DispatchToKindle(disp DispatchItem) (bool, error) {
	var target MyKindleDestination
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

	e := email.NewEmail()
	log.Printf("Emailing %s to %s %s", cont.Path, target.To, target.Type())

	settings := target.Target.SendCredentials
	e.From = settings.From
	e.To = []string{target.To}
	e.Bcc = []string{settings.From}
	e.Subject = fmt.Sprintf("%s: %s", disp.Item.Feed.Title, disp.Item.Title)
	if _, err := e.AttachFile(cont.Path); err != nil {
		return false, err
	}

	err := e.Send(settings.Server+":"+settings.Port, smtp.PlainAuth("", settings.User, settings.Password, settings.Server))
	if err != nil {
		return false, err
	}
	return true, nil
}
