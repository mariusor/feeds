package feeds

import (
	"github.com/bmaupin/go-epub"
)

func ToEPub(content []byte, title string, author string, outPath string) error {
	e := epub.NewEpub(title)

	e.SetAuthor(author)

	e.AddSection(string(content), title, "", "")

	// Write the EPUB
	if err := e.Write(outPath); err != nil {
		return err
	}
	return nil
}
