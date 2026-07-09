package httpapi

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
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
	User struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
}

func registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "fail",
			"message": "Malformed request body",
		})
		return
	}

	var errors []string
	if req.User.Email == "" {
		errors = append(errors, "Email can't be blank")
	}
	if req.User.Name == "" {
		errors = append(errors, "Name can't be blank")
	}
	if req.User.Password == "" {
		errors = append(errors, "Password can't be blank")
	}
	if req.User.PasswordConfirmation == "" {
		errors = append(errors, "Password confirmation can't be blank")
	}
	if req.User.Password != "" && req.User.PasswordConfirmation != "" && req.User.Password != req.User.PasswordConfirmation {
		errors = append(errors, "Password confirmation doesn't match Password")
	}

	if len(errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors": errors,
		})
		return
	}

	// Generate a Devise-compatible bcrypt hash. Persistence is introduced in later PRs.
	_, _ = bcrypt.GenerateFromPassword([]byte(req.User.Password), bcrypt.DefaultCost)

	var resp registerResponse
	resp.User.ID = 1
	resp.User.Email = req.User.Email
	resp.User.Name = req.User.Name

	writeJSON(w, http.StatusCreated, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

