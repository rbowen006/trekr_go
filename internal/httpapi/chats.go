package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/services"
	"gorm.io/gorm"
)

func (app *App) chatService() *services.ChatService {
	return &services.ChatService{DB: app.DB}
}

type messageParamsRequest struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

// chatsCreate serves POST /api/v1/listings/:listing_id/chats — find-or-create
// the hirer's unbooked chat with the listing owner and append the first message.
// Returns 201 for a new chat, 200 for an existing one (mirrors the controller).
func (app *App) chatsCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "listing_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}
	var listing models.RvListing
	if err := app.DB.First(&listing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
		http.Error(w, "could not load listing", http.StatusInternalServerError)
		return
	}

	var req messageParamsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}

	chat, created, validationErrors, err := app.chatService().CreateFromListing(user.ID, &listing, req.Message.Content)
	if err != nil {
		http.Error(w, "could not create chat", http.StatusInternalServerError)
		return
	}
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, serializeChat(chat, true, false))
}

// chatsIndex serves GET /api/v1/chats — the user's chats split into as_hirer and
// as_owner, each newest-active first with participants.
func (app *App) chatsIndex(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	svc := app.chatService()
	asHirer, err := svc.ChatsForRole(user.ID, "hirer")
	if err != nil {
		http.Error(w, "could not load chats", http.StatusInternalServerError)
		return
	}
	asOwner, err := svc.ChatsForRole(user.ID, "owner")
	if err != nil {
		http.Error(w, "could not load chats", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"as_hirer": serializeChatList(asHirer),
		"as_owner": serializeChatList(asOwner),
	})
}

// chatsShow serves GET /api/v1/chats/:id — full chat (messages + participants)
// for a participant; 404 for a missing chat, 403 for a non-participant.
func (app *App) chatsShow(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	chat, ok := app.loadChat(w, r, "id")
	if !ok {
		return
	}
	if !isParticipant(chat, user.ID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return
	}

	writeJSON(w, http.StatusOK, serializeChat(chat, true, true))
}

// messagesCreate serves POST /api/v1/chats/:chat_id/messages — a participant
// posts a message; 403 for a non-participant, 422 for blank content.
func (app *App) messagesCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	chat, ok := app.loadChatRow(w, r, "chat_id")
	if !ok {
		return
	}
	if !isParticipant(chat, user.ID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return
	}

	var req messageParamsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}

	message, validationErrors, err := app.chatService().CreateMessage(chat.ID, user.ID, req.Message.Content)
	if err != nil {
		http.Error(w, "could not create message", http.StatusInternalServerError)
		return
	}
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}
	writeJSON(w, http.StatusCreated, serializeMessage(message))
}

// messagesIndex serves GET /api/v1/chats/:chat_id/messages — a participant lists
// the chat's messages (oldest-first). Marks the other participant's messages as
// read before rendering, so read_at reflects the update (mirrors the
// controller's lazy relation render after mark_messages_read).
func (app *App) messagesIndex(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	chat, ok := app.loadChatRow(w, r, "chat_id")
	if !ok {
		return
	}
	if !isParticipant(chat, user.ID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return
	}

	svc := app.chatService()
	if err := svc.MarkRead(chat, user.ID); err != nil {
		http.Error(w, "could not update messages", http.StatusInternalServerError)
		return
	}
	messages, err := svc.ListMessages(chat.ID)
	if err != nil {
		http.Error(w, "could not load messages", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, serializeMessages(messages))
}

// loadChat loads the chat named by the {param} URL segment with its hirer,
// owner, listing, and ordered messages, for the full show payload.
func (app *App) loadChat(w http.ResponseWriter, r *http.Request, param string) (*models.Chat, bool) {
	return app.loadChatWith(w, r, param, func(db *gorm.DB) *gorm.DB {
		return db.
			Preload("Hirer").Preload("Owner").Preload("RvListing").
			Preload("Messages", func(db *gorm.DB) *gorm.DB { return db.Order("created_at") })
	})
}

// loadChatRow loads just the chat row (no associations), enough for the message
// endpoints' participant check; they load messages separately.
func (app *App) loadChatRow(w http.ResponseWriter, r *http.Request, param string) (*models.Chat, bool) {
	return app.loadChatWith(w, r, param, nil)
}

// loadChatWith loads the chat named by the {param} URL segment, applying the
// optional scope for preloads. It writes a 404 and returns ok=false when the
// chat is missing (Rails' Chat.find raises RecordNotFound => 404).
func (app *App) loadChatWith(w http.ResponseWriter, r *http.Request, param string, scope func(*gorm.DB) *gorm.DB) (*models.Chat, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return nil, false
	}
	query := app.DB
	if scope != nil {
		query = scope(query)
	}
	var chat models.Chat
	err = query.First(&chat, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return nil, false
	}
	if err != nil {
		http.Error(w, "could not load chat", http.StatusInternalServerError)
		return nil, false
	}
	return &chat, true
}

func isParticipant(chat *models.Chat, userID int64) bool {
	return chat.HirerID == userID || chat.OwnerID == userID
}

func serializeChatList(chats []models.Chat) []chatJSON {
	out := make([]chatJSON, len(chats))
	for i := range chats {
		out[i] = serializeChat(&chats[i], false, true)
	}
	return out
}
