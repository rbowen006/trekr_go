package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/rbowen/trekr_go/internal/auth"
	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

// resetPasswordWithin mirrors Devise config.reset_password_within (6.hours).
const resetPasswordWithin = 6 * time.Hour

const resetInstructionsMessage = "If that email is registered you will receive a reset link shortly."

type passwordResetCreateRequest struct {
	User struct {
		Email string `json:"email"`
	} `json:"user"`
}

type passwordResetUpdateRequest struct {
	User struct {
		ResetPasswordToken   string `json:"reset_password_token"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	} `json:"user"`
}

// createPasswordReset issues a Devise-compatible reset token for a registered
// email. It always returns 200 with a generic message so callers cannot probe
// which emails exist.
func (app *App) createPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetCreateRequest
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
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// No such user: no side effect, but respond identically.
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not process request",
		})
		return
	default:
		if err := app.issueResetToken(&user); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"status":  "fail",
				"message": "Could not process request",
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": resetInstructionsMessage})
}

func (app *App) issueResetToken(user *models.User) error {
	raw, err := auth.GenerateResetToken()
	if err != nil {
		return err
	}
	digest, err := auth.ResetTokenDigest(app.Config.SecretKeyBase, raw)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := app.DB.Model(user).Updates(map[string]any{
		"reset_password_token":   digest,
		"reset_password_sent_at": now,
	}).Error; err != nil {
		return err
	}
	// No mailer in dev: log the reset link target so the flow is exercisable.
	log.Printf("[dev] password reset requested for %s (token=%s)", user.Email, raw)
	return nil
}

// updatePasswordReset consumes a reset token and sets a new password, mirroring
// Devise.reset_password_by_token.
func (app *App) updatePasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "fail",
			"message": "Malformed request body",
		})
		return
	}

	digest, err := auth.ResetTokenDigest(app.Config.SecretKeyBase, req.User.ResetPasswordToken)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not process request",
		})
		return
	}

	var user models.User
	err = app.DB.Where("reset_password_token = ?", digest).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors": []string{"Reset password token is invalid"},
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not process request",
		})
		return
	}

	if user.ResetPasswordSentAt == nil || time.Since(*user.ResetPasswordSentAt) > resetPasswordWithin {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors": []string{"Reset password token has expired, please request a new one"},
		})
		return
	}

	if verrs := validateNewPassword(req.User.Password, req.User.PasswordConfirmation); len(verrs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": verrs})
		return
	}

	hash, err := auth.HashPassword(req.User.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not process request",
		})
		return
	}

	if err := app.DB.Model(&user).Updates(map[string]any{
		"encrypted_password":     hash,
		"reset_password_token":   nil,
		"reset_password_sent_at": nil,
	}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "fail",
			"message": "Could not process request",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully."})
}

func validateNewPassword(password, confirmation string) []string {
	var errs []string
	if password == "" {
		errs = append(errs, "Password can't be blank")
		return errs
	}
	if len(password) < 6 {
		errs = append(errs, "Password is too short (minimum is 6 characters)")
	}
	if password != confirmation {
		errs = append(errs, "Password confirmation doesn't match Password")
	}
	return errs
}
