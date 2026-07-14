// Package jobs holds the asynq task definitions, the enqueue client, and the
// task handlers, mirroring rv_marketplace's app/jobs. Redis-backed asynq stands
// in for ActiveJob (ADR-0011).
package jobs

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

// TypeListingEmbed is the asynq task type for regenerating a listing's
// semantic-search embedding (mirrors GenerateListingEmbeddingJob).
const TypeListingEmbed = "listing:embed"

// ListingEmbedPayload carries the listing id to (re)embed.
type ListingEmbedPayload struct {
	RvListingID int64 `json:"rv_listing_id"`
}

// Client enqueues jobs onto the asynq/Redis queue. It satisfies the
// services.ListingEmbedQueue interface consumed by ListingService.
type Client struct {
	client *asynq.Client
}

// NewClient connects an asynq client to Redis at redisURL (redis://host:port/db).
func NewClient(redisURL string) (*Client, error) {
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, err
	}
	return &Client{client: asynq.NewClient(opt)}, nil
}

// Close releases the underlying Redis connection.
func (c *Client) Close() error { return c.client.Close() }

// EnqueueListingEmbed enqueues a listing-embed task. Mirrors
// GenerateListingEmbeddingJob.perform_later(id).
func (c *Client) EnqueueListingEmbed(listingID int64) error {
	payload, err := json.Marshal(ListingEmbedPayload{RvListingID: listingID})
	if err != nil {
		return err
	}
	_, err = c.client.Enqueue(asynq.NewTask(TypeListingEmbed, payload))
	return err
}
