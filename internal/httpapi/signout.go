package httpapi

import "net/http"

// signOut mirrors Devise's sign-out under the Null JWT revocation strategy: it
// is a stateless no-op that always returns 204, tolerating missing or malformed
// Authorization headers so the frontend can sign out without crashing.
func (app *App) signOut(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
