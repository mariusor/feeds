package feeds

import (
	"net/url"
	"time"
)

type Feed struct {
	ID        int
	URL       url.URL
	Title     string
	Frequency time.Duration
	Updated   time.Time
}

type Item struct {
	ID        int
	URL       url.URL
	FeedID    int
	GUID      string
	Title     string
	Author    string
	Published time.Time
	Updated   time.Time
	Status    int
}

type Content struct {
	ID         int
	URL        url.URL
	ItemID     int
	HTMLPath   string
	MobiPath   string
	Dispatched bool
}
