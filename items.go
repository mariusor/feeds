package feeds

import (
	"fmt"
	"net/url"
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
