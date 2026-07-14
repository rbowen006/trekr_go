//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// --- helpers --------------------------------------------------------------

func seedChat(t *testing.T, app *httpapi.App, hirerID, ownerID, listingID int64) *models.Chat {
	t.Helper()
	lid := listingID
	c := &models.Chat{HirerID: hirerID, OwnerID: ownerID, RvListingID: &lid}
	require.NoError(t, app.DB.Create(c).Error)
	return c
}

// seedMessage inserts a message and mirrors Message#update_chat_inbox_fields
// (after_create_commit) so the chat's inbox columns reflect the latest message,
// matching what the Rails factory triggers.
func seedMessage(t *testing.T, app *httpapi.App, chatID, userID int64, content string) *models.Message {
	t.Helper()
	m := &models.Message{ChatID: chatID, UserID: userID, Content: content}
	require.NoError(t, app.DB.Create(m).Error)
	require.NoError(t, app.DB.Model(&models.Chat{}).
		Where("id = ? AND (last_message_at IS NULL OR last_message_at < ?)", chatID, m.CreatedAt).
		Updates(map[string]any{"last_message_at": m.CreatedAt, "last_message_content": content}).Error)
	return m
}

func messageBody(content string) string {
	return fmt.Sprintf(`{"message":{"content":%q}}`, content)
}

// objectKeys returns the top-level keys of a JSON object in document order, so
// tests can lock the serializer's field order against Rails' as_json.
func objectKeys(t *testing.T, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok)

	var keys []string
	depth := 0
	for dec.More() || depth > 0 {
		tk, err := dec.Token()
		require.NoError(t, err)
		switch v := tk.(type) {
		case json.Delim:
			if v == '{' || v == '[' {
				depth++
			} else {
				depth--
			}
		case string:
			if depth == 0 {
				keys = append(keys, v)
				// Skip the value for this key.
				skipValue(t, dec)
			}
		}
	}
	return keys
}

func skipValue(t *testing.T, dec *json.Decoder) {
	t.Helper()
	tk, err := dec.Token()
	require.NoError(t, err)
	if d, ok := tk.(json.Delim); ok && (d == '{' || d == '[') {
		depth := 1
		for depth > 0 {
			nt, err := dec.Token()
			require.NoError(t, err)
			if dd, ok := nt.(json.Delim); ok {
				if dd == '{' || dd == '[' {
					depth++
				} else {
					depth--
				}
			}
		}
	}
}

// --- POST /api/v1/listings/:listing_id/chats ------------------------------

func TestChatCreate_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "ch-auth")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/chats", server.URL, listing.ID), "", messageBody("Hi"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestChatCreate_CreatesChatAndFirstMessage(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "ch-o")
	hirer := seedOwner(t, app, "ch-h")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/chats", server.URL, listing.ID),
		testutil.AuthHeader(t, app, hirer), messageBody("Is this available in July?"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		HirerID     int64 `json:"hirer_id"`
		OwnerID     int64 `json:"owner_id"`
		RvListingID int64 `json:"rv_listing_id"`
		Messages    []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, hirer.ID, got.HirerID)
	require.Equal(t, owner.ID, got.OwnerID)
	require.Equal(t, listing.ID, got.RvListingID)
	require.Len(t, got.Messages, 1)
	require.Equal(t, "Is this available in July?", got.Messages[0].Content)
}

func TestChatCreate_ReturnsExistingChatAndUpdatesSubject(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "ch-eo")
	hirer := seedOwner(t, app, "ch-eh")
	listing := seedListing(t, app, owner.ID, "100")
	otherListing := seedListing(t, app, owner.ID, "150")
	existing := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/chats", server.URL, otherListing.ID),
		testutil.AuthHeader(t, app, hirer), messageBody("What about this one?"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		ID          int64 `json:"id"`
		RvListingID int64 `json:"rv_listing_id"`
		Messages    []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, existing.ID, got.ID)
	require.Equal(t, otherListing.ID, got.RvListingID)
	require.NotEmpty(t, got.Messages)
	require.Equal(t, "What about this one?", got.Messages[len(got.Messages)-1].Content)
}

// --- GET /api/v1/chats ----------------------------------------------------

func TestChatsIndex_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp := doAuthJSON(t, http.MethodGet, server.URL+"/api/v1/chats", "", "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestChatsIndex_SplitsAsHirerAndAsOwner(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "ix-o")
	hirer := seedOwner(t, app, "ix-h")
	listing := seedListing(t, app, owner.ID, "100")
	chatAsHirer := seedChat(t, app, hirer.ID, owner.ID, listing.ID)
	seedMessage(t, app, chatAsHirer.ID, hirer.ID, "Latest message")

	resp := doAuthJSON(t, http.MethodGet, server.URL+"/api/v1/chats", testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		AsHirer []map[string]json.RawMessage `json:"as_hirer"`
		AsOwner []map[string]json.RawMessage `json:"as_owner"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Empty(t, got.AsOwner)

	var row map[string]json.RawMessage
	for _, c := range got.AsHirer {
		var id int64
		require.NoError(t, json.Unmarshal(c["id"], &id))
		if id == chatAsHirer.ID {
			row = c
		}
	}
	require.NotNil(t, row, "chat should appear under as_hirer")

	var content string
	require.NoError(t, json.Unmarshal(row["last_message_content"], &content))
	require.Equal(t, "Latest message", content)
	require.Contains(t, row, "last_message_at")
	require.Contains(t, row, "hirer_last_read_at")
	require.Contains(t, row, "owner_last_read_at")
	// Index omits messages (include_participants only).
	require.NotContains(t, row, "messages")
	require.Contains(t, row, "listing_title")
}

// --- GET /api/v1/chats/:id ------------------------------------------------

func TestChatShow_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "sh-o")
	hirer := seedOwner(t, app, "sh-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d", server.URL, chat.ID), "", "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestChatShow_ParticipantSucceeds(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "sp-o")
	hirer := seedOwner(t, app, "sp-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d", server.URL, chat.ID),
		testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, chat.ID, got.ID)
	// Show includes messages and participants.
	keys := objectKeys(t, body)
	require.Subset(t, keys, []string{"messages", "hirer", "owner", "listing_title"})
}

func TestChatShow_NonParticipantForbidden(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "nf-o")
	hirer := seedOwner(t, app, "nf-h")
	outsider := seedOwner(t, app, "nf-x")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d", server.URL, chat.ID),
		testutil.AuthHeader(t, app, outsider), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestChatShow_MissingReturnsNotFound(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	hirer := seedOwner(t, app, "mn-h")

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d", server.URL, int64(999999999)),
		testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- POST /api/v1/chats/:chat_id/messages ---------------------------------

func TestMessageCreate_HirerSucceeds(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mh-o")
	hirer := seedOwner(t, app, "mh-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, hirer), messageBody("Any chance of a discount?"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, []string{"id", "content", "user_id", "chat_id", "read_at", "created_at"}, objectKeys(t, body))

	var got struct {
		Content string `json:"content"`
		UserID  int64  `json:"user_id"`
		ChatID  int64  `json:"chat_id"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "Any chance of a discount?", got.Content)
	require.Equal(t, hirer.ID, got.UserID)
	require.Equal(t, chat.ID, got.ChatID)
}

func TestMessageCreate_CreatedAtRendersUTC(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mu-o")
	hirer := seedOwner(t, app, "mu-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	before := time.Now().UTC().Add(-2 * time.Second)
	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, hirer), messageBody("hello"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		CreatedAt string `json:"created_at"`
	}
	require.NoError(t, json.Unmarshal(body, &got))

	// Rails renders as_json timestamps as UTC with a "Z" suffix and 3 fractional
	// digits; the value must be the true instant (not shifted by the host zone).
	require.True(t, strings.HasSuffix(got.CreatedAt, "Z"), "created_at should be UTC: %s", got.CreatedAt)
	parsed, err := time.Parse("2006-01-02T15:04:05.000Z07:00", got.CreatedAt)
	require.NoError(t, err)
	require.WithinDuration(t, before.Add(2*time.Second), parsed, 5*time.Second,
		"created_at %s should be near now (UTC), not shifted by host timezone", got.CreatedAt)
}

func TestMessageCreate_OwnerReplies(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mo-o")
	hirer := seedOwner(t, app, "mo-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, owner), messageBody("Sure, 10% off for you!"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestMessageCreate_ThirdPartyForbidden(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mt-o")
	hirer := seedOwner(t, app, "mt-h")
	outsider := seedOwner(t, app, "mt-x")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, outsider), messageBody("I am not in this chat"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestMessageCreate_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "ma-o")
	hirer := seedOwner(t, app, "ma-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID), "", messageBody("Hello"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestMessageCreate_BlankContentUnprocessable(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mb-o")
	hirer := seedOwner(t, app, "mb-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, hirer), messageBody(""))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Contains(t, got.Errors, "Content can't be blank")
}

// --- GET /api/v1/chats/:chat_id/messages ----------------------------------

func TestMessagesIndex_HirerLists(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "li-o")
	hirer := seedOwner(t, app, "li-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)
	msg := seedMessage(t, app, chat.ID, hirer.ID, "First")

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got []struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	ids := make([]int64, len(got))
	for i, m := range got {
		ids[i] = m.ID
	}
	require.Contains(t, ids, msg.ID)
}

func TestMessagesIndex_OwnerLists(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "lo-o")
	hirer := seedOwner(t, app, "lo-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)
	seedMessage(t, app, chat.ID, hirer.ID, "First")

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMessagesIndex_MarksOtherParticipantMessagesRead(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mr-o")
	hirer := seedOwner(t, app, "mr-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)
	ownerMsg := seedMessage(t, app, chat.ID, owner.ID, "Hi from owner")
	require.Nil(t, ownerMsg.ReadAt)

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID),
		testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var reloaded models.Message
	require.NoError(t, app.DB.First(&reloaded, ownerMsg.ID).Error)
	require.NotNil(t, reloaded.ReadAt)

	var reloadedChat models.Chat
	require.NoError(t, app.DB.First(&reloadedChat, chat.ID).Error)
	require.NotNil(t, reloadedChat.HirerLastReadAt)
}

func TestMessagesIndex_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "lr-o")
	hirer := seedOwner(t, app, "lr-h")
	listing := seedListing(t, app, owner.ID, "100")
	chat := seedChat(t, app, hirer.ID, owner.ID, listing.ID)

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/chats/%d/messages", server.URL, chat.ID), "", "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
