package feeds

import (
	"github.com/mariusor/go-readability"
)

func toReadableHtml(content []byte) ([]byte, string, error) {
	var err error
	var doc *readability.Document

	doc, err = readability.NewDocument(string(content))
	doc.WhitelistTags = append(doc.WhitelistTags, "h1", "h2", "h3", "h4", "h5", "h6", "img", "hr")
	doc.RemoveUnlikelyCandidates = true
	doc.EnsureTitleInArticle = true
	doc.WhitelistAttrs["img"] = []string{"src", "title", "alt"}
	doc.WhitelistAttrs["p"] = []string{"style"}
	if err != nil {
		return []byte{}, "err::title", err
	}

	return []byte(doc.Content()), doc.Title, nil
}
