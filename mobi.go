package feeds

import (
	"github.com/766b/mobi"
)

const HtmlDir = "articles"
const MobiDir = "output/mobi"
const OutputDir = "output"

func ToMobi(content []byte, title string, author string, outPath string) error {
	m, err := mobi.NewWriter(outPath)
	if err != nil {
		return err
	}

	m.Title(title)
	m.Compression(mobi.CompressionHuffCdic)
	m.NewExthRecord(mobi.EXTH_DOCTYPE, "PBOK")
	if author != "" {
		m.NewExthRecord(mobi.EXTH_AUTHOR, author)
	}
	m.NewChapter(title, content)
	// Output MOBI File
	m.Write()
	return nil
}
