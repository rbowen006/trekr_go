package ai

import (
	"context"

	"github.com/rbowen/trekr_go/internal/models"
)

// Chat-reply limits mirror Ai::ChatReplySuggester's MAX_MESSAGES /
// MAX_MESSAGE_LENGTH: only the most recent messages are sent, each truncated.
const (
	MaxChatMessages  = 10
	MaxMessageLength = 500
)

// SuggestReply runs the chat-reply feature for a chat, logging one ai_requests
// row attributed to userID. The chat must have its Messages (oldest-first) and
// RvListing preloaded by the caller. Mirrors Ai::ChatReplySuggester.call.
func (c *Claude) SuggestReply(ctx context.Context, chat *models.Chat, userID *int64) (map[string]any, error) {
	return c.run(ctx, &chatReplySuggester{chat: chat}, userID)
}

// chatReplySuggester is the per-feature seam for owner reply drafting
// (Ai::ChatReplySuggester).
type chatReplySuggester struct {
	chat *models.Chat
}

func (g *chatReplySuggester) Feature() string       { return "chat_reply" }
func (g *chatReplySuggester) PromptVersion() string { return "v1" }

func (g *chatReplySuggester) ValidateInput() error {
	if g.chat == nil {
		return &InputError{Msg: "Missing chat"}
	}
	return nil
}

type conversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// listingFacts fixes the field order of the listing block to match the Ruby hash
// in ChatReplySuggester#listing_facts.
type listingFacts struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	RvType      string  `json:"rv_type"`
	Town        string  `json:"town"`
	State       string  `json:"state"`
	MaxGuests   int     `json:"max_guests"`
	PetFriendly bool    `json:"pet_friendly"`
	PricePerDay *string `json:"price_per_day"`
}

// chatUserMessage fixes the top-level order (perspective, messages, listing) to
// match ChatReplySuggester#build_user_message, where listing is inserted last
// and only when the chat is tied to a listing.
type chatUserMessage struct {
	Perspective string                `json:"perspective"`
	Messages    []conversationMessage `json:"messages"`
	Listing     *listingFacts         `json:"listing,omitempty"`
}

func (g *chatReplySuggester) BuildUserMessage() (any, error) {
	msg := chatUserMessage{
		Perspective: "owner",
		Messages:    g.conversation(),
	}
	if g.chat.RvListing != nil {
		l := g.chat.RvListing
		msg.Listing = &listingFacts{
			Title:       l.Title,
			Description: l.Description,
			RvType:      models.RvTypeName(l.RvType),
			Town:        l.Town,
			State:       l.State,
			MaxGuests:   l.MaxGuests,
			PetFriendly: l.PetFriendly,
			PricePerDay: l.PricePerDay,
		}
	}
	return msg, nil
}

// conversation returns the most recent MaxChatMessages messages, oldest-first,
// each labelled hirer/owner and truncated — mirroring #recent_messages +
// #conversation. chat.Messages is preloaded oldest-first, so the tail is the
// newest window.
func (g *chatReplySuggester) conversation() []conversationMessage {
	all := g.chat.Messages
	if len(all) > MaxChatMessages {
		all = all[len(all)-MaxChatMessages:]
	}
	out := make([]conversationMessage, len(all))
	for i := range all {
		out[i] = conversationMessage{
			Role:    g.roleFor(all[i]),
			Content: truncate(all[i].Content, MaxMessageLength, "…"),
		}
	}
	return out
}

func (g *chatReplySuggester) roleFor(m models.Message) string {
	if m.UserID == g.chat.HirerID {
		return "hirer"
	}
	return "owner"
}

func (g *chatReplySuggester) ValidateOutput(data map[string]any) error {
	return requireNonEmptyString(data, "reply")
}

// truncate mirrors ActiveSupport's String#truncate(length, omission:): if the
// string is longer than length (counted in characters), keep the first
// (length - len(omission)) characters and append omission. Counts runes, not
// bytes, matching Ruby's character semantics.
func truncate(s string, length int, omission string) string {
	runes := []rune(s)
	if len(runes) <= length {
		return s
	}
	stop := length - len([]rune(omission))
	if stop < 0 {
		stop = 0
	}
	return string(runes[:stop]) + omission
}
