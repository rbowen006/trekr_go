package httpapi

import (
	"encoding/json"
	"time"

	"github.com/rbowen/trekr_go/internal/models"
)

// railsTimeLayout renders timestamps as Rails' as_json does: ISO-8601 in UTC
// with 3 fractional digits and a "Z" suffix (default time_zone UTC, default
// time_precision 3), e.g. "2026-07-14T06:07:26.123Z".
const railsTimeLayout = "2006-01-02T15:04:05.000Z07:00"

func formatRailsTime(t time.Time) string {
	return t.UTC().Format(railsTimeLayout)
}

func formatRailsTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatRailsTime(*t)
	return &s
}

// chatJSON matches Chat#as_json. The base fields (id..owner_last_read_at) are
// always rendered (nullable columns as JSON null). The optional sections follow
// Rails' key order: messages (include_messages), then hirer/owner/listing_title
// (include_participants); each is a pointer so it is omitted when not requested.
type chatJSON struct {
	ID                 int64          `json:"id"`
	HirerID            int64          `json:"hirer_id"`
	OwnerID            int64          `json:"owner_id"`
	RvListingID        *int64         `json:"rv_listing_id"`
	BookingID          *int64         `json:"booking_id"`
	LastMessageAt      *string        `json:"last_message_at"`
	LastMessageContent *string        `json:"last_message_content"`
	HirerLastReadAt    *string        `json:"hirer_last_read_at"`
	OwnerLastReadAt    *string        `json:"owner_last_read_at"`
	Messages           *[]messageJSON `json:"messages,omitempty"`
	Hirer              *ownerJSON     `json:"hirer,omitempty"`
	Owner              *ownerJSON     `json:"owner,omitempty"`
	// ListingTitle is rv_listing&.title: present (string or JSON null) when
	// participants are included, omitted otherwise. RawMessage lets a
	// listing-less chat render null while a present-but-empty title renders "".
	ListingTitle json.RawMessage `json:"listing_title,omitempty"`
}

// messageJSON matches Message#as_json: only [id, content, user_id, chat_id,
// read_at, created_at], in that order.
type messageJSON struct {
	ID        int64   `json:"id"`
	Content   string  `json:"content"`
	UserID    int64   `json:"user_id"`
	ChatID    int64   `json:"chat_id"`
	ReadAt    *string `json:"read_at"`
	CreatedAt string  `json:"created_at"`
}

func serializeMessage(m *models.Message) messageJSON {
	return messageJSON{
		ID:        m.ID,
		Content:   m.Content,
		UserID:    m.UserID,
		ChatID:    m.ChatID,
		ReadAt:    formatRailsTimePtr(m.ReadAt),
		CreatedAt: formatRailsTime(m.CreatedAt),
	}
}

func serializeMessages(msgs []models.Message) []messageJSON {
	out := make([]messageJSON, len(msgs))
	for i := range msgs {
		out[i] = serializeMessage(&msgs[i])
	}
	return out
}

// serializeChat builds the chat payload. includeMessages appends the ordered
// messages array; includeParticipants appends hirer/owner/listing_title. This
// mirrors the flag combinations the controller uses: create (messages only),
// index (participants only), show (both).
func serializeChat(c *models.Chat, includeMessages, includeParticipants bool) chatJSON {
	out := chatJSON{
		ID:                 c.ID,
		HirerID:            c.HirerID,
		OwnerID:            c.OwnerID,
		RvListingID:        c.RvListingID,
		BookingID:          c.BookingID,
		LastMessageAt:      formatRailsTimePtr(c.LastMessageAt),
		LastMessageContent: c.LastMessageContent,
		HirerLastReadAt:    formatRailsTimePtr(c.HirerLastReadAt),
		OwnerLastReadAt:    formatRailsTimePtr(c.OwnerLastReadAt),
	}

	if includeMessages {
		msgs := serializeMessages(c.Messages)
		out.Messages = &msgs
	}

	if includeParticipants {
		if c.Hirer != nil {
			out.Hirer = &ownerJSON{ID: c.Hirer.ID, Name: c.Hirer.Name}
		}
		if c.Owner != nil {
			out.Owner = &ownerJSON{ID: c.Owner.ID, Name: c.Owner.Name}
		}
		if c.RvListing != nil {
			out.ListingTitle, _ = json.Marshal(c.RvListing.Title)
		} else {
			out.ListingTitle = json.RawMessage("null")
		}
	}

	return out
}
