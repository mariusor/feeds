package internal

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"git.sr.ht/~mariusor/ssm"
	"github.com/mariusor/feeds"
)

type SM struct {
	Path string
	db   *sql.DB
}

const defaultPollTime = 10 * time.Second

func (s SM) Wait(ctx context.Context) (ssm.Fn, error) {
	db, err := feeds.DB(s.Path)
	if err != nil {
		return ssm.End, fmt.Errorf("failed to open database: %w", err)
	}
	s.db = db
	defer s.db.Close()

	next := ssm.Every(defaultPollTime, ssm.Batch(s.FetchFeeds, s.FetchItems, s.GenerateContent, s.DispatchContent))

	return next(ctx)
}

func (s SM) FetchFeeds(ctx context.Context) (ssm.Fn, error) {
	_, err := FetchFeeds(ctx, s.db)
	if err != nil {
		return ssm.End, err
	}

	return s.Wait, nil
}

func (s SM) FetchItems(ctx context.Context) (ssm.Fn, error) {
	_, err := FetchItems(ctx, s.db, s.Path)
	if err != nil {
		return ssm.End, err
	}

	return s.Wait, nil
}

func (s SM) GenerateContent(ctx context.Context) (ssm.Fn, error) {
	if err := GenerateContent(ctx, s.db, s.Path); err != nil {
		return ssm.End, err
	}

	return s.Wait, nil
}
func (s SM) DispatchContent(ctx context.Context) (ssm.Fn, error) {
	if err := DispatchContent(ctx, s.db); err != nil {
		return ssm.End, err
	}

	return s.Wait, nil
}
