package feeds

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	halfDay                = time.Hour * 12
	chunkSize              = 10
	defaultSleepAfterBatch = 200 * time.Millisecond
)

func FetchItemsCmd(ctx context.Context, c *sql.DB, basePath string) (bool, error) {
	all, err := GetNonFetchedItems(c)
	if err != nil {
		return false, err
	}
	if len(all) == 0 {
		log.Printf("No items found for fetching")
		return false, nil
	}
	status := false
	maxFailureCount := 3
	failures := make(map[int]int)
	m := sync.Mutex{}
	g, _ := errgroup.WithContext(ctx)
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			it := all[j]
			if failures[it.Feed.ID] > maxFailureCount {
				log.Printf("Skipping %s, too many failures when loading", it.URL)
				continue
			}
			g.Go(func() error {
				defer func() {
					m.Unlock()
					time.Sleep(defaultSleepAfterBatch)
				}()

				m.Lock()
				status, err = LoadItem(&it, c, basePath)
				if err != nil {
					log.Printf("Error[%5d] %s %s", it.FeedIndex, it.URL.String(), err.Error())
					failures[it.Feed.ID]++
				}
				log.Printf("Loaded[%5d] %s [%t]", it.FeedIndex, it.URL.String(), status)

				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return status, err
		}
	}
	return status, nil
}

func FetchFeedsCmd(ctx context.Context, c *sql.DB) (bool, error) {
	all, err := GetFeeds(c)
	if err != nil {
		return false, err
	}
	if len(all) == 0 {
		return false, nil
	}

	hasNewItems := false
	g, _ := errgroup.WithContext(ctx)
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			f := all[j]
			if f.URL == nil {
				continue
			}

			g.Go(func() error {
				if f.URL.Scheme == "" {
					log.Printf("Feed %s has an invalid URL, skipping...", f.Title)
					return nil
				}
				log.Printf("Feed %s\n", f.URL.String())
				if f.Frequency == 0 {
					f.Frequency = halfDay
				}
				var last time.Duration = 0
				if !f.Updated.IsZero() {
					last = time.Now().UTC().Sub(f.Updated)
					log.Printf("Last checked %s ago", last.Round(10*time.Second).String())
				}
				if last > 0 && last <= f.Frequency {
					log.Printf(" ...newer than %s, skipping.\n", f.Frequency.String())
					return nil
				}

				hasItems := false
				if hasItems, err = CheckFeed(f, c); err != nil {
					log.Printf("Error: %s", err)
				}
				hasNewItems = hasNewItems || hasItems
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return hasNewItems, err
		}
	}
	return hasNewItems, nil
}

func GenerateContentCmd(ctx context.Context, c *sql.DB, basePath string) error {
	all, err := GetContentsForEbook(c, ValidEbookTypes[:]...)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		log.Printf("No content found for generating ebook versions")
		return nil
	}

	m := sync.Mutex{}
	g, _ := errgroup.WithContext(ctx)
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			item := &all[j]
			g.Go(func() error {
				defer m.Unlock()

				m.Lock()
				gen, err := generateContent(item, basePath, true)
				if err != nil {
					MarkItemsAsFailed(c, *item)
					return nil
				}

				if gen {
					if err = InsertContent(c, *item); err != nil {
						log.Printf("Unable to update paths in db: %s", err.Error())
						return nil
					}
					log.Printf("Updated content items [%d] %s: %v", item.ID, item.Title, item.Content)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func generateContent(item *Item, basePath string, overwrite bool) (bool, error) {
	generated := false
	if gen, err := GenerateContent(OutputTypeHTML, basePath, item, overwrite); err != nil {
		log.Printf("Unable to generate path: %s", err.Error())
		if errors.Is(err, FileSizeError) {
			return gen, err
		}
		generated = generated || gen
	}

	errs := make([]error, 0)
	for _, typ := range ValidEbookTypes {
		if c, ok := item.Content[typ]; ok {
			if fileExists(c.Path) {
				continue
			}
			delete(item.Content, typ)
		}
		gen, err := GenerateContent(typ, basePath, item, overwrite)
		if err != nil {
			log.Printf("Unable to generate path: %s", err.Error())
			errs = append(errs, err)
		}
		generated = generated || gen
	}
	return generated, errors.Join(errs...)
}

func DispatchContentCmd(ctx context.Context, c *sql.DB) error {
	all, err := GetNonDispatchedItemContentsForDestination(c)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		log.Printf("No content found for dispatch")
		return nil
	}

	maxFailureCount := 3
	failures := make(map[int]int)
	m := sync.Mutex{}

	g, _ := errgroup.WithContext(ctx)
	for i := 0; i < len(all); i += chunkSize {
		for j := i; j < i+chunkSize && j < len(all); j++ {
			disp := all[j]
			if failures[disp.Destination.ID] > maxFailureCount {
				log.Printf("Skipping destination %s[%d], too many failures when dispatching", disp.Destination.Type, disp.Destination.ID)
				continue
			}
			g.Go(func() error {
				defer func() {
					m.Unlock()
					time.Sleep(defaultSleepAfterBatch)
				}()
				m.Lock()
				if err := dispatch(c, disp); err != nil {
					log.Printf("Error: %s", err.Error())
					failures[disp.Destination.ID]++
					return err
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func dispatch(c *sql.DB, disp DispatchItem) error {
	var err error
	var status bool

	switch disp.Destination.Type {
	case "myk":
		status, err = DispatchToKindle(disp)
	case "pocket":
		status, err = DispatchToPocket(disp)
	default:
		return fmt.Errorf("unknown dispatch type %s", disp.Destination.Type)
	}
	disp.LastStatus = status
	if err != nil {
		disp.LastMessage = err.Error()
	}
	SaveTarget(c, disp)
	return err
}
