package feeds

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Item struct {
	ID        int
	FeedIndex int
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
	return fmt.Sprintf("%05d %s.%s", i.FeedIndex, strings.TrimSpace(i.Title), ext)
}

func Slug(s string) string {
	s = strings.Map(func (r rune) rune {
		switch r {
		case ',', '?', '!', '\'', '`':
			return -1
		}
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'z') {
			return r
		}
		return '-'
	}, strings.ToLower(s))
	rr := regexp.MustCompile("-+")
	b := rr.ReplaceAll([]byte(s), []byte{'-'})
	if b[len(b)-1] == '-' {
		b = b[:len(b)-1]
	}
	return string(b)
}


func (i Item) PathSlug() string {
	return fmt.Sprintf("%d-%s", i.FeedIndex, Slug(i.Title))
}
