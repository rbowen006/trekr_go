package httpapi

import (
	"net/http"

	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
	"github.com/rbowen/trekr_go/internal/models"
)

// suggestReply serves POST /api/v1/chats/:id/suggest_reply — an authed,
// rate-limited call that drafts the owner's next reply via Claude. Mirrors
// Api::V1::ChatReplySuggesterController#create: 404 for a missing chat, 403
// unless the caller owns the chat, 422 until a hirer message exists, then the
// AI call. Mapping order matters — none of these paths call Claude.
func (app *App) suggestReply(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	chat, ok := app.loadChat(w, r, "chat_id")
	if !ok {
		return
	}
	if chat.OwnerID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"status": "fail", "message": "Forbidden"})
		return
	}
	if !hasHirerMessage(chat) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"status": "fail", "message": "No hirer message to reply to yet."})
		return
	}

	data, err := app.Claude.SuggestReply(r.Context(), chat, &user.ID)
	if err != nil {
		writeAiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "data": data})
}

// hasHirerMessage reports whether the chat has at least one message from the
// hirer, mirroring chat.messages.exists?(user_id: chat.hirer_id).
func hasHirerMessage(chat *models.Chat) bool {
	for _, m := range chat.Messages {
		if m.UserID == chat.HirerID {
			return true
		}
	}
	return false
}
