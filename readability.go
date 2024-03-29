package feeds

import (
	"os"
	"path"

	"github.com/mariusor/go-readability"
)

func Readability(content []byte) (*readability.Document, error) {
	doc, err := readability.NewDocument(string(content))
	if err != nil {
		return nil, err
	}
	doc.RemoveUnlikelyCandidates = true
	doc.EnsureTitleInArticle = true
	doc.WhitelistTags = append(doc.WhitelistTags, "h1", "h2", "h3", "h4", "h5", "h6", "img", "hr", "br")
	doc.WhitelistTags = append(doc.WhitelistTags, "table", "tr", "td", "th", "tbody", "tcol")
	doc.WhitelistTags = append(doc.WhitelistTags, "blockquote", "article", "section")
	doc.WhitelistTags = append(doc.WhitelistTags, "ul", "ol", "li", "dl", "dt", "dd")
	doc.WhitelistAttrs["img"] = []string{"src", "title", "alt"}
	doc.MinTextLength = 10
	//doc.WhitelistAttrs["p"] = []string{"style"}
	return doc, nil
}

func ToReadableHtml(content []byte, title, author, outPath string) error {
	doc, err := Readability(content)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path.Dir(outPath)); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(path.Dir(outPath), 0755); err != nil {
			return err
		}
	}
	cont := doc.Content()
	if err = os.WriteFile(outPath, []byte(cont), 0644); err != nil {
		return err
	}
	return nil
}
