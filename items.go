package feeds

import (
	"net/url"
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
	Dispatched bool
	Item       Item
}
