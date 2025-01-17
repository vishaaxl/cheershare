package data

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type Creative struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"-"`
	CreativeURL string    `json:"creative_url"`
	ScheduledAt time.Time `json:"scheduled_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreativeModel struct {
	DB *sql.DB
}

func (c *CreativeModel) Insert(creative *Creative) error {
	query := `INSERT INTO creatives (user_id, creative_url, scheduled_at)
			VALUES ($1, $2, $3)
			RETURNING id, created_at`

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	args := []interface{}{creative.UserID, creative.CreativeURL, creative.ScheduledAt}
	err := c.DB.QueryRowContext(ctx, query, args...).Scan(&creative.ID, &creative.CreatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (c *CreativeModel) GetScheduledCreatives() (map[string][]Creative, error) {
	query := `
		SELECT id, user_id, creative_url, scheduled_at, created_at 
		FROM creatives 
		WHERE scheduled_at = ANY($1)
	`

	dates := []time.Time{
		time.Now().Truncate(24 * time.Hour),
		time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	rows, err := c.DB.QueryContext(ctx, query, pq.Array(dates))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	creatives := map[string][]Creative{
		"today":    {},
		"tomorrow": {},
	}

	for rows.Next() {
		var creative Creative
		err := rows.Scan(&creative.ID, &creative.UserID, &creative.CreativeURL, &creative.ScheduledAt, &creative.CreatedAt)
		if err != nil {
			return nil, err
		}

		if creative.ScheduledAt.Equal(dates[0]) {
			creatives["today"] = append(creatives["today"], creative)
		} else if creative.ScheduledAt.Equal(dates[1]) {
			creatives["tomorrow"] = append(creatives["tomorrow"], creative)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return creatives, nil
}
