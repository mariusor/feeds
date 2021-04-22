package feeds

import (
	"github.com/mariusor/go-readability"
	"io/ioutil"
	"os"
	"path"
)

func ToReadableHtml(content []byte, outPath string) error {
	doc, err := readability.NewDocument(string(content))
	if err != nil {
		return err
	}
	doc.WhitelistTags = append(doc.WhitelistTags, "h1", "h2", "h3", "h4", "h5", "h6", "img", "hr")
	doc.RemoveUnlikelyCandidates = true
	doc.EnsureTitleInArticle = true
	doc.WhitelistAttrs["img"] = []string{"src", "title", "alt"}
	//doc.WhitelistAttrs["p"] = []string{"style"}

	if _, err := os.Stat(path.Dir(outPath)); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(path.Dir(outPath), 0755); err != nil {
			return err
		}
	}
	if err = ioutil.WriteFile(outPath, content, 0644); err != nil {
		return err
	}
	return nil
}
