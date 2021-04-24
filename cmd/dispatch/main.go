package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/mariusor/feeds"
	"golang.org/x/sync/errgroup"
)

const (
	chunkSize              = 5
	defaultSleepAfterBatch = 200 * time.Millisecond
)

func main() {
	var (
		basePath string
		verbose  bool
	)
	flag.StringVar(&basePath, "path", "/tmp", "Base path")
	flag.BoolVar(&verbose, "verbose", false, "Output debugging messages")
	flag.Parse()

	basePath = path.Clean(basePath)
	htmlBasePath := path.Join(basePath, feeds.HtmlDir)
	if _, err := os.Stat(htmlBasePath); os.IsNotExist(err) {
		// fail if can't load html folder
		log.Fatalf("Invalid html folder %s", htmlBasePath)
	}
	mobiBasePath := path.Dir(path.Join(basePath, feeds.MobiDir))
	if _, err := os.Stat(mobiBasePath); os.IsNotExist(err) {
		os.Mkdir(mobiBasePath, 0755)
	}

	c, err := feeds.DB(basePath)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer c.Close()

	all, err := feeds.GetNonDispatchedItemContentsForDestination(c)
	if err != nil {
		log.Printf("Error: %s", err)
	}

	maxFailureCount := 3
	failures := make(map[int]int)
	m := sync.Mutex{}

	g, _ := errgroup.WithContext(context.Background())
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
				if err := Dispatch(c, disp); err != nil {
					log.Printf("Error: %s", err.Error())
					failures[disp.Destination.ID]++
					return err
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}

func Dispatch(c *sql.DB, disp feeds.DispatchItem) error {
	var (
		err    error
		status bool
	)
	switch disp.Destination.Type {
	case "myk":
		status, err = feeds.DispatchToKindle(disp)
	case "pocket":
		status, err = feeds.DispatchToPocket(disp)
	default:
		return fmt.Errorf("unknown dispatch type %s", disp.Destination.Type)
	}
	disp.LastStatus = status
	if err != nil {
		disp.LastMessage = err.Error()
	}
	feeds.SaveTarget(c, disp)
	return err
}
