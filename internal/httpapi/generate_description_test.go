//go:build integration

package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rbowen/trekr_go/internal/ai"
	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// stubClaude points the app's Claude runner at a fake Anthropic Messages API.
// On 2xx it returns an assistant message whose single text block is innerText
// (the JSON the service parses); otherwise it returns the given error status. It
// reports how many upstream calls were served, so tests can assert a throttled
// request never reaches Claude. Mirrors the WebMock stub in the Rails specs.
func stubClaude(t *testing.T, app *httpapi.App, innerText string, status int) *int {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"type":"overloaded_error","message":"Overloaded"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_123", "type": "message", "role": "assistant",
			"content":     []map[string]any{{"type": "text", "text": innerText}},
			"model":       "claude-sonnet-4-6",
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 120, "output_tokens": 30},
		})
	}))
	t.Cleanup(srv.Close)
	app.Claude = ai.NewClaude(app.DB, "test-key", srv.URL)
	return &calls
}

func descriptionBody() string {
	return `{"rv_type":"caravan","town":"Byron Bay","state":"NSW","max_guests":4,"pet_friendly":true,"price_per_day":180}`
}

const generateDescriptionURL = "/api/v1/listings/generate_description"

func TestGenerateDescription_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"x"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, "", descriptionBody())
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGenerateDescription_ReturnsJSendSuccess(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"A stunning caravan in Byron Bay."}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-ok")
	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, testutil.AuthHeader(t, app, user), descriptionBody())
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Status string `json:"status"`
		Data   struct {
			Description string `json:"description"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "success", got.Status)
	require.Equal(t, "A stunning caravan in Byron Bay.", got.Data.Description)
}

func TestGenerateDescription_MissingRequiredFieldIsFail(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"x"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-fail")
	// Omit rv_type -> InputError -> 400 fail.
	body := `{"town":"Byron Bay","state":"NSW","max_guests":4,"pet_friendly":true,"price_per_day":180}`
	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, testutil.AuthHeader(t, app, user), body)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var got struct {
		Status string `json:"status"`
	}
	raw, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "fail", got.Status)
}

func TestGenerateDescription_ClaudeFailureIsError(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, "", 529) // overloaded
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-529")
	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, testutil.AuthHeader(t, app, user), descriptionBody())
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	var got struct {
		Status string `json:"status"`
	}
	raw, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "error", got.Status)
}

// --- ai_requests logging (BaseAiService ensure-block parity) ---------------

func TestGenerateDescription_LogsAiRequestOnSuccess(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"Logged."}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-log")
	before := aiRequestCount(t, app, "description_generator")

	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, testutil.AuthHeader(t, app, user), descriptionBody())
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, before+1, aiRequestCount(t, app, "description_generator"))

	var rec models.AiRequest
	require.NoError(t, app.DB.Where("feature = ?", "description_generator").Order("id DESC").First(&rec).Error)
	require.True(t, rec.Success)
	require.Equal(t, "claude-sonnet-4-6", rec.Model)
	require.NotNil(t, rec.UserID)
	require.Equal(t, user.ID, *rec.UserID)
	require.NotNil(t, rec.InputTokens)
	require.Equal(t, 120, *rec.InputTokens)
	require.NotNil(t, rec.EstimatedCostUSD)
}

func TestGenerateDescription_LogsAiRequestOnApiFailure(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, "", 529)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-log-fail")
	before := aiRequestCount(t, app, "description_generator")

	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, testutil.AuthHeader(t, app, user), descriptionBody())
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	require.Equal(t, before+1, aiRequestCount(t, app, "description_generator"))

	// A failed Claude call logs a row with success=false and nil tokens/cost.
	var rec models.AiRequest
	require.NoError(t, app.DB.Where("feature = ?", "description_generator").Order("id DESC").First(&rec).Error)
	require.False(t, rec.Success)
	require.Nil(t, rec.InputTokens)
	require.Nil(t, rec.EstimatedCostUSD)
	require.NotNil(t, rec.ErrorMessage)
}

// --- rate limiting (generate_description_rate_limit_spec.rb) ---------------

func TestGenerateDescription_Allows10Blocks11th(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"ok"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-rl")
	auth := testutil.AuthHeader(t, app, user)

	for i := 0; i < 10; i++ {
		resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, auth, descriptionBody())
		require.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	}

	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, auth, descriptionBody())
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestGenerateDescription_ThrottledBodyAndRetryAfter(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"ok"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-rl-body")
	auth := testutil.AuthHeader(t, app, user)

	var resp *http.Response
	for i := 0; i < 11; i++ {
		if resp != nil {
			_ = resp.Body.Close()
		}
		resp = doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, auth, descriptionBody())
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

func TestGenerateDescription_LimitsPerUser(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"ok"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-rl-u1")
	auth := testutil.AuthHeader(t, app, user)
	var resp *http.Response
	for i := 0; i < 11; i++ {
		if resp != nil {
			_ = resp.Body.Close()
		}
		resp = doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, auth, descriptionBody())
	}
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	_ = resp.Body.Close()

	other := seedOwner(t, app, "gen-rl-u2")
	resp = doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, testutil.AuthHeader(t, app, other), descriptionBody())
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGenerateDescription_ThrottledRequestDoesNotCallClaude(t *testing.T) {
	app := testutil.NewTestApp(t)
	calls := stubClaude(t, app, `{"description":"ok"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-rl-calls")
	auth := testutil.AuthHeader(t, app, user)
	for i := 0; i < 11; i++ {
		resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, auth, descriptionBody())
		_ = resp.Body.Close()
	}
	// Only the 10 admitted requests reach Anthropic; the throttled 11th does not.
	require.Equal(t, 10, *calls)
}

func TestGenerateDescription_ValidationFailuresCountAgainstLimit(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubClaude(t, app, `{"description":"ok"}`, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "gen-rl-inval")
	auth := testutil.AuthHeader(t, app, user)
	// The limiter increments on admission, before validation: ten 400s exhaust
	// the window; the 11th is throttled (429), not another 400.
	invalid := `{"town":"Byron Bay","state":"NSW","max_guests":4}`
	for i := 0; i < 10; i++ {
		resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, auth, invalid)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		_ = resp.Body.Close()
	}
	resp := doAuthJSON(t, http.MethodPost, server.URL+generateDescriptionURL, auth, invalid)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}
