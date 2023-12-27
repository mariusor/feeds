package feeds

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	HtmlDir   = "articles"
	OutputDir = "output"

	OutputTypeRAW  = "raw"
	OutputTypeHTML = "html"
	OutputTypeEPUB = "epub"
	OutputTypeMOBI = "mobi"
	OutputTypeAZW3 = "azw3"
)

var ValidEbookTypes = [...]string{
	OutputTypeHTML,
	OutputTypeEPUB,
	OutputTypeMOBI,
	OutputTypeAZW3,
}

type convertFn func(content []byte, title string, author string, outPath string) error

type itemTypeFn func(*Item, string) convertFn

type typeDependency map[string]string

type functionMapper map[string]convertFn

var typesDependencies = typeDependency{
	OutputTypeMOBI: OutputTypeHTML,
	OutputTypeEPUB: OutputTypeHTML,
	OutputTypeAZW3: OutputTypeHTML,
	OutputTypeHTML: OutputTypeRAW,
}

var typesFunctions = functionMapper{
	OutputTypeMOBI: ToMobi,
	OutputTypeEPUB: ToEPub,
	OutputTypeAZW3: ToAZW3,
	OutputTypeHTML: ToReadableHtml,
}

var mappings = map[string]itemTypeFn{
	OutputTypeMOBI: ebook,
	OutputTypeEPUB: ebook,
	OutputTypeAZW3: ebook,
	OutputTypeHTML: html,
}

func validEbookType(typ string) bool {
	for _, t := range ValidEbookTypes {
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

var FileSizeError = fmt.Errorf("file size is smaller than 20%% of average of existing ones")

func ebook(item *Item, typ string) convertFn {
	fn := typesFunctions[typ]
	return func(content []byte, title string, author string, outPath string) error {
		if err := fn(content, title, author, outPath); err != nil {
			return err
		}
		item.Content[typ] = Content{Path: outPath, Type: typ}
		return nil
	}
}

func html(item *Item, typ string) convertFn {
	fn := typesFunctions[typ]
	return func(content []byte, title string, author string, outPath string) error {
		if err := fn(content, title, author, outPath); err != nil {
			return err
		}
		if avgSize := feedItemsAverageSize(outPath); len(content)*5 < avgSize {
			return FileSizeError
		}
		item.Content[typ] = Content{Path: outPath, Type: typ}
		return nil
	}
}

func getItemContentForType(it Item, typ string) ([]byte, error) {
	c, ok := it.Content[typ]
	if !ok {
		return nil, fmt.Errorf("invalid content of type %s for item %v", typ, it)
	}
	buf, err := readFile(c.Path)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func GenerateContent(typ, basePath string, item *Item, overwrite bool) (bool, error) {
	if !validEbookType(typ) {
		return false, fmt.Errorf("invalid ebook type %s, valid ones are %v", typ, ValidEbookTypes)
	}
	if c, ok := item.Content[typ]; ok && c.Path != "" {
		return false, nil
	}
	var err error

	outPath := path.Join(basePath, OutputDir, strings.TrimSpace(item.Feed.Title), typ, item.Path(typ))
	if !path.IsAbs(outPath) {
		if outPath, err = filepath.Abs(outPath); err != nil {
			return false, err
		}
	}
	outDirPath := path.Dir(outPath)
	if _, err := os.Stat(outDirPath); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(outDirPath, 0755); err != nil {
			return false, err
		}
	}
	fi, err := os.Stat(outPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if ((err != nil && os.IsNotExist(err)) || fi.ModTime().Sub(time.Now()).Truncate(time.Second) == 0) && !overwrite {
		return false, nil
	}
	buf, err := getItemContentForType(*item, typesDependencies[typ])
	if err != nil {
		return false, err
	}
	fn := mappings[typ](item, typ)
	if err = fn(buf, strings.TrimSpace(item.Title), strings.TrimSpace(item.Author), outPath); err != nil {
		return false, err
	}
	if _, err := os.Stat(outPath); err != nil {
		return false, err
	}

	return true, nil
}
