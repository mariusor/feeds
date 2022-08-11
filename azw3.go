package feeds

import (
	"github.com/leotaku/mobi"
	"os"
)

func ToAZW3(content []byte, title, author, outPath string) error {
	b := mobi.Book{
		Title: title,
		Chapters: []mobi.Chapter{
			{
				Title:  title,
				Chunks: mobi.Chunks(string(content)),
			},
		},
	}
	if author != "" {
		b.Authors = []string{author}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return b.Realize().Write(f)
}
