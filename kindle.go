package feeds

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/smtp"

	"github.com/jordan-wright/email"
	_ "modernc.org/sqlite"
)

var DefaultMyKindleSender = SMTPCreds{
	Server:   "smtp.example.com",
	Port:     "587",
	From:     "feedsync@example.com",
	User:     "FeedSync",
	Password: "",
}

var Kindle = Target{
	ID:    1,
	Type:  "kindle",
	Flags: 0,
}

type SMTPCreds struct {
	Server   string `json:"server"`
	Port     string `json:"port"`
	From     string `json:"from"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type myKindleDestination struct {
	To string `json:"to"`
}

func DispatchToKindle(subject string, attachment string, c *sql.DB) (bool, error) {
	if attachment == "" {
		return false, errors.New("missing attachment file")
	}
	targets := "SELECT targets.data, outputs.credentials FROM targets INNER JOIN outputs ON outputs.id = targets.output_id WHERE outputs.type = 'kindle'"
	r, err := c.Query(targets)
	if err != nil {
		return false, err
	}
	for r.Next() {
		var data []byte
		var credentials []byte

		r.Scan(&data, &credentials)

		var settings SMTPCreds
		var target myKindleDestination
		err = json.Unmarshal(credentials, &settings)
		if err != nil {
			return false, err
		}
		err = json.Unmarshal(data, &target)
		if err != nil {
			return false, err
		}
		e := email.NewEmail()
		log.Printf("Emailing %s to %s", attachment, target.To)

		e.From = settings.From
		e.To = []string{target.To}
		e.Bcc = []string{settings.From}
		e.Subject = subject
		e.AttachFile(attachment)
		err = e.Send(settings.Server+":"+settings.Port, smtp.PlainAuth("", settings.User, settings.Password, settings.Server))
		if err != nil {
			return false, err
		}
	}
	return true, nil
}
