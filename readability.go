package feeds

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/mariusor/go-readability"
)

func ToReadableHtml(content []byte, outPath string) error {
	doc, err := readability.NewDocument(string(content))
	if err != nil {
		return err
	}
	doc.RemoveUnlikelyCandidates = true
	doc.EnsureTitleInArticle = true
	doc.WhitelistTags = append(doc.WhitelistTags, "h1", "h2", "h3", "h4", "h5", "h6", "img", "hr")
	doc.WhitelistTags = append(doc.WhitelistTags, "table", "tr", "td", "th", "tbody", "tcol")
	doc.WhitelistAttrs["img"] = []string{"src", "title", "alt"}
	//doc.WhitelistAttrs["p"] = []string{"style"}

	if _, err := os.Stat(path.Dir(outPath)); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(path.Dir(outPath), 0755); err != nil {
			return err
		}
	}
	cont := doc.Content()
	if err = ioutil.WriteFile(outPath, []byte(cont), 0644); err != nil {
		return err
	}
	return nil
}
