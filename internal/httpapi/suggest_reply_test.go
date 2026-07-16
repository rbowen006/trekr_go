//go:build integration

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// suggestChat seeds an owner/hirer/listing/chat and, when withHirerMsg is set, a
// hirer message so a reply can be suggested.
func suggestChat(t *testing.T, app *httpapi.App, tag string, withHirerMsg bool) (owner, hirer *models.User, chatID int64) {
	t.Helper()
	owner = seedOwner(t, app, tag+"-o")
	hirer = seedOwner(t, app, tag+"-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)
	if withHirerMsg {
		seedMessage(t, app, chat.ID, hirer.ID, "How many does it sleep?")
	}
	return owner, hirer, chat.ID
}

func TestSuggestReply_ReturnsJSendSuccessForOwner(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"reply":"Yes, it sleeps four comfortably."}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner, _, chatID := suggestChat(t, app, "sr-ok", true)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)
	resp := doAuthJSON(t, http.MethodPost, url, testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Status string `json:"status"`
		Data   struct {
			Reply string `json:"reply"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "success", got.Status)
	require.Equal(t, "Yes, it sleeps four comfortably.", got.Data.Reply)
}

func TestSuggestReply_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"reply":"x"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	_, _, chatID := suggestChat(t, app, "sr-auth", true)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)
	resp := doAuthJSON(t, http.MethodPost, url, "", "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSuggestReply_ForbidsHirer(t *testing.T) {
	app := testutil.NewTestApp(t)
	calls := stubClaude(t, app, `{"reply":"x"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	_, hirer, chatID := suggestChat(t, app, "sr-forbid", true)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)
	resp := doAuthJSON(t, http.MethodPost, url, testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, 0, *calls, "a forbidden request must not call Claude")
}

func TestSuggestReply_MissingChatReturnsNotFound(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"reply":"x"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "sr-404")
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, int64(999999999))
	resp := doAuthJSON(t, http.MethodPost, url, testutil.AuthHeader(t, app, user), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSuggestReply_NoHirerMessageUnprocessable(t *testing.T) {
	app := testutil.NewTestApp(t)
	calls := stubClaude(t, app, `{"reply":"x"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	// Chat exists but only the owner has spoken -> nothing to reply to.
	owner, _, chatID := suggestChat(t, app, "sr-422", false)
	seedMessage(t, app, chatID, owner.ID, "Hi, are you still interested?")
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)
	resp := doAuthJSON(t, http.MethodPost, url, testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "fail", got.Status)
	require.Equal(t, 0, *calls, "a 422 must not call Claude")
}

func TestSuggestReply_ClaudeUnavailableIsError(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, "", 529)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner, _, chatID := suggestChat(t, app, "sr-503", true)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)
	resp := doAuthJSON(t, http.MethodPost, url, testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "error", got.Status)
}

// --- rate limiting (suggest_reply_rate_limit_spec.rb) ---------------------

func TestSuggestReply_Allows10Blocks11th(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"reply":"Sure thing!"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner, _, chatID := suggestChat(t, app, "sr-rl", true)
	auth := testutil.AuthHeader(t, app, owner)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)

	for i := 0; i < 10; i++ {
		resp := doAuthJSON(t, http.MethodPost, url, auth, "")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	}
	resp := doAuthJSON(t, http.MethodPost, url, auth, "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestSuggestReply_ThrottledBodyAndRetryAfter(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"reply":"Sure thing!"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner, _, chatID := suggestChat(t, app, "sr-rl-body", true)
	auth := testutil.AuthHeader(t, app, owner)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)

	var resp *http.Response
	for i := 0; i < 11; i++ {
		if resp != nil {
			_ = resp.Body.Close()
		}
		resp = doAuthJSON(t, http.MethodPost, url, auth, "")
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, "3600", resp.Header.Get("Retry-After"))

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "fail", got.Status)
	require.NotEmpty(t, got.Message)
}

func TestSuggestReply_LimitsPerUser(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"reply":"Sure thing!"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner, _, chatID := suggestChat(t, app, "sr-rl-u1", true)
	auth := testutil.AuthHeader(t, app, owner)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)
	var resp *http.Response
	for i := 0; i < 11; i++ {
		if resp != nil {
			_ = resp.Body.Close()
		}
		resp = doAuthJSON(t, http.MethodPost, url, auth, "")
	}
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	_ = resp.Body.Close()

	// A different owner with their own chat is unaffected.
	otherOwner, _, otherChatID := suggestChat(t, app, "sr-rl-u2", true)
	otherURL := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, otherChatID)
	resp = doAuthJSON(t, http.MethodPost, otherURL, testutil.AuthHeader(t, app, otherOwner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSuggestReply_ThrottledRequestDoesNotCallClaude(t *testing.T) {
	app := testutil.NewTestApp(t)
	calls := stubClaude(t, app, `{"reply":"Sure thing!"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner, _, chatID := suggestChat(t, app, "sr-rl-calls", true)
	auth := testutil.AuthHeader(t, app, owner)
	url := fmt.Sprintf("%s/api/v1/chats/%d/suggest_reply", server.URL, chatID)
	for i := 0; i < 11; i++ {
		resp := doAuthJSON(t, http.MethodPost, url, auth, "")
		_ = resp.Body.Close()
	}
	require.Equal(t, 10, *calls)
}
