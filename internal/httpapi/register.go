package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rbowen/trekr_go/internal/auth"
	"github.com/rbowen/trekr_go/internal/models"
)

type registerRequest struct {
	User struct {
		Email                string `json:"email"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
		Name                 string `json:"name"`
	} `json:"user"`
}

type registerResponse struct {
	User models.User `json:"user"`
}

func (app *App) registerUser(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "fail",
			"message": "Malformed request body",
		})
		return
	}

	if validationErrors := validateRegisterRequest(req); len(validationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors": validationErrors,
		})
		return
	}

	hash, err := auth.HashPassword(req.User.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not create user",
		})
		return
	}

	user := models.User{
		Email:             normalizeEmail(req.User.Email),
		Name:              strings.TrimSpace(req.User.Name),
		EncryptedPassword: hash,
	}

	if err := app.DB.Create(&user).Error; err != nil {
		if isDuplicateEmailError(err) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"errors": []string{"Email has already been taken"},
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not create user",
		})
		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{User: user})
}

func validateRegisterRequest(req registerRequest) []string {
	var errs []string
	if strings.TrimSpace(req.User.Email) == "" {
		errs = append(errs, "Email can't be blank")
	}
	if strings.TrimSpace(req.User.Name) == "" {
		errs = append(errs, "Name can't be blank")
	}
	if req.User.Password == "" {
		errs = append(errs, "Password can't be blank")
	}
	if req.User.PasswordConfirmation == "" {
		errs = append(errs, "Password confirmation can't be blank")
	}
	if req.User.Password != "" && len(req.User.Password) < 6 {
		errs = append(errs, "Password is too short (minimum is 6 characters)")
	}
	if req.User.Password != "" && req.User.PasswordConfirmation != "" && req.User.Password != req.User.PasswordConfirmation {
		errs = append(errs, "Password confirmation doesn't match Password")
	}
	return errs
}

func isDuplicateEmailError(err error) bool {
	return strings.Contains(err.Error(), "duplicate key") ||
		strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "idx_users_on_email")
}

// normalizeEmail matches Devise's case_insensitive_keys/strip_whitespace_keys
// handling so registration and sign-in agree on the stored/looked-up value.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	// json.Marshal matches Rails' render json: byte-for-byte — it HTML-escapes
	// <, >, & (Rails escape_html_entities_in_json is true) and adds no trailing
	// newline (unlike json.Encoder).
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
