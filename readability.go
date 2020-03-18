package feeds

import (
	"github.com/mariusor/go-readability"
)

func toReadableHtml(content []byte) ([]byte, string, error) {
	var err error
	var doc *readability.Document

	doc, err = readability.NewDocument(string(content))
	if err != nil {
		return []byte{}, "err::title", err
	}

	return []byte(doc.Content()), doc.Title, nil
}
