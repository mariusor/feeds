package feeds

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var validEbookTypes = [...]string{
	"html",
	"epub",
	"mobi",
}

func validEbookType(typ string) bool {
	for _, t := range validEbookTypes {
		if t == typ {
			return true
		}
	}
	return false
}

func readFile(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var size int
	if info, err := f.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}
	size++ // one byte for final read at EOF

	// If a file claims a small size, read at least 512 bytes.
	// In particular, files in Linux's /proc claim size 0 but
	// then do not work right if read in small pieces,
	// so an initial read of 1 byte would not work correctly.
	if size < 512 {
		size = 512
	}

	data := make([]byte, 0, size)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}
func GenerateContent(typ, basePath string, item *Item, overwrite bool) error {
	if !validEbookType(typ) {
		return fmt.Errorf("invalid ebook type %s, valid ones are %v", typ, validEbookTypes)
	}
	if c, ok := item.Content[typ]; ok && c.Path != "" {
		return nil
	}
	var (
		ebookFn func(content []byte, title string, author string, outPath string) error
		contFn  func() ([]byte, error)
		err     error
	)
	needsHtmlFn := func(typ string) func() ([]byte, error) {
		return func() ([]byte, error) {
			c, ok := item.Content[typ]
			if !ok {
				return nil, fmt.Errorf("invalid content of type %s for item %v", typ, item)
			}
			buf, err := readFile(c.Path)
			if err != nil {
				return nil, err
			}
			return buf, nil
		}
	}
	switch typ {
	case "mobi":
		contFn = needsHtmlFn("html")
		ebookFn = func(content []byte, title string, author string, outPath string) error {
			if err := ToMobi(content, title, author, outPath); err != nil {
				return err
			}
			item.Content["mobi"] = Content{Path: outPath, Type: "mobi"}
			return nil
		}
	case "epub":
		contFn = needsHtmlFn("html")
		ebookFn = func(content []byte, title string, author string, outPath string) error {
			if err := ToEPub(content, title, author, outPath); err != nil {
				return err
			}
			item.Content["epub"] = Content{Path: outPath, Type: "epub"}
			return nil
		}
	case "html":
		contFn = needsHtmlFn("raw")
		ebookFn = func(content []byte, title string, author string, outPath string) error {
			if err = ToReadableHtml(content, outPath); err != nil {
				return err
			}
			item.Content["html"] = Content{Path: outPath, Type: "html"}
			return nil
		}
	}
	ebookPath := path.Join(basePath, OutputDir, strings.TrimSpace(item.Feed.Title), typ, item.Path(typ))
	if !path.IsAbs(ebookPath) {
		if ebookPath, err = filepath.Abs(ebookPath); err != nil {
			return err
		}
	}
	ebookDirPath := path.Dir(ebookPath)
	if _, err := os.Stat(ebookDirPath); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(ebookDirPath, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(ebookPath); !overwrite && !(err != nil && os.IsNotExist(err)) {
		return nil
	}
	buf, err := contFn()
	if err != nil {
		return err
	}
	if err = ebookFn(buf, strings.TrimSpace(item.Title), strings.TrimSpace(item.Author), ebookPath); err != nil {
		return err
	}
	if _, err := os.Stat(ebookPath); err != nil {
		return err
	}

	return nil
}
