package services

import (
	"errors"
	"strings"
	"time"

	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

// ChatService owns chat/message persistence: find-or-create of the single
// unbooked chat per (hirer, owner) pair, message creation with inbox-field
// upkeep, and read-state tracking. Mirrors ChatsController + MessagesController
// and the Chat/Message model callbacks.
type ChatService struct {
	DB *gorm.DB
}

// CreateFromListing mirrors ChatsController#create: it finds the hirer's
// existing unbooked chat with the listing's owner (booking_id IS NULL) or opens
// a new one, updates the chat's subject to this listing, and appends the first
// message. created reports whether a new chat was opened (=> 201 vs 200). The
// returned chat has its Messages preloaded (ordered) for the response. A blank
// message yields Rails-style validation messages (=> 422).
func (s *ChatService) CreateFromListing(hirerID int64, listing *models.RvListing, content string) (chat *models.Chat, created bool, validationErrors []string, err error) {
	var existing models.Chat
	findErr := s.DB.
		Where("hirer_id = ? AND owner_id = ? AND booking_id IS NULL", hirerID, listing.OwnerID).
		First(&existing).Error

	lid := listing.ID
	switch {
	case errors.Is(findErr, gorm.ErrRecordNotFound):
		existing = models.Chat{HirerID: hirerID, OwnerID: listing.OwnerID, RvListingID: &lid}
		if e := s.DB.Create(&existing).Error; e != nil {
			return nil, false, nil, e
		}
		created = true
	case findErr != nil:
		return nil, false, nil, findErr
	default:
		if e := s.DB.Model(&existing).Update("rv_listing_id", lid).Error; e != nil {
			return nil, false, nil, e
		}
		existing.RvListingID = &lid
	}

	_, verrs, e := s.createMessage(existing.ID, hirerID, content)
	if e != nil {
		return nil, false, nil, e
	}
	if len(verrs) > 0 {
		return nil, false, verrs, nil
	}

	if e := s.DB.Preload("Messages", orderByCreatedAt).First(&existing, existing.ID).Error; e != nil {
		return nil, false, nil, e
	}
	return &existing, created, nil, nil
}

// CreateMessage mirrors MessagesController#create: it appends a message from the
// given user to the chat, returning Rails-style validation messages for blank
// content (=> 422).
func (s *ChatService) CreateMessage(chatID, userID int64, content string) (*models.Message, []string, error) {
	return s.createMessage(chatID, userID, content)
}

// createMessage persists a message and updates the chat's inbox columns when the
// message is newer, mirroring Message#update_chat_inbox_fields
// (after_create_commit).
func (s *ChatService) createMessage(chatID, userID int64, content string) (*models.Message, []string, error) {
	if strings.TrimSpace(content) == "" {
		return nil, []string{"Content can't be blank"}, nil
	}

	msg := &models.Message{ChatID: chatID, UserID: userID, Content: content}
	if err := s.DB.Create(msg).Error; err != nil {
		return nil, nil, err
	}

	if err := s.DB.Model(&models.Chat{}).
		Where("id = ? AND (last_message_at IS NULL OR last_message_at < ?)", chatID, msg.CreatedAt).
		Updates(map[string]any{"last_message_at": msg.CreatedAt, "last_message_content": msg.Content}).
		Error; err != nil {
		return nil, nil, err
	}
	return msg, nil, nil
}

// ChatsForRole returns the chats where the user is the hirer (role "hirer") or
// the owner (role "owner"), newest-active first, with participants and listing
// preloaded for the index payload. Mirrors ChatsController#chats_for_role.
func (s *ChatService) ChatsForRole(userID int64, role string) ([]models.Chat, error) {
	column := "owner_id"
	if role == "hirer" {
		column = "hirer_id"
	}

	var chats []models.Chat
	err := s.DB.
		Where(column+" = ?", userID).
		Order("last_message_at DESC").
		Preload("Hirer").Preload("Owner").Preload("RvListing").
		Find(&chats).Error
	return chats, err
}

// ListMessages returns the chat's messages ordered oldest-first.
func (s *ChatService) ListMessages(chatID int64) ([]models.Message, error) {
	var messages []models.Message
	err := s.DB.Where("chat_id = ?", chatID).Order("created_at").Find(&messages).Error
	return messages, err
}

// MarkRead mirrors MessagesController#mark_messages_read: it marks the other
// participant's unread messages as read and stamps the reader's last-read
// timestamp on the chat.
func (s *ChatService) MarkRead(chat *models.Chat, userID int64) error {
	now := time.Now().UTC()
	if err := s.DB.Model(&models.Message{}).
		Where("chat_id = ? AND user_id <> ? AND read_at IS NULL", chat.ID, userID).
		Update("read_at", now).Error; err != nil {
		return err
	}

	column := "owner_last_read_at"
	if chat.HirerID == userID {
		column = "hirer_last_read_at"
	}
	return s.DB.Model(&models.Chat{}).Where("id = ?", chat.ID).Update(column, now).Error
}

func orderByCreatedAt(db *gorm.DB) *gorm.DB {
	return db.Order("created_at")
}
