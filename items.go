package feeds

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Item struct {
	ID        int
	URL       *url.URL
	GUID      string
	Title     string
	Author    string
	Published time.Time
	Updated   time.Time
	Status    int
	Feed      Feed
}

type Content struct {
	ID         int
	URL        *url.URL
	HTMLPath   string
	MobiPath   string
	EPubPath   string
	Dispatched bool
	Item       Item
}

func (i Item) Path(ext string) string {
	return fmt.Sprintf("%05d %s.%s", i.ID, strings.TrimSpace(i.Title), ext)
}

func (c Content) HTML() ([]byte, error) {
	f, err := os.Open(c.HTMLPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
