package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rbowen/trekr_go/internal/auth"
	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

type signInRequest struct {
	User struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	} `json:"user"`
}

// signIn authenticates a user and, on success, returns the user JSON with a
// Devise-compatible JWT in the Authorization header.
func (app *App) signIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "fail",
			"message": "Malformed request body",
		})
		return
	}

	email := normalizeEmail(req.User.Email)

	var user models.User
	err := app.DB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || !auth.VerifyPassword(req.User.Password, user.EncryptedPassword) {
		writeInvalidCredentials(w)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not sign in",
		})
		return
	}

	token, err := auth.IssueToken(app.Config.SecretKeyBase, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not sign in",
		})
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)

	writeJSON(w, http.StatusOK, registerResponse{User: user})
}

// writeInvalidCredentials mirrors Rails' JsendFailureApp for a failed sign-in.
func writeInvalidCredentials(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{
		"status":  "fail",
		"message": "Invalid Email or password.",
	})
}
