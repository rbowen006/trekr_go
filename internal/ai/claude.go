package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/prompts"
	"gorm.io/gorm"
)

// DefaultModel and DefaultMaxTokens mirror BaseAiService's DEFAULT_MODEL /
// DEFAULT_MAX_TOKENS. The model string is a contract value written to
// ai_requests.model and priced in pricing.go; do not swap it for an SDK default.
const (
	DefaultModel     = "claude-sonnet-4-6"
	DefaultMaxTokens = 1024
)

// generator is the per-feature seam of BaseAiService: the parts that differ
// between the description generator and the chat-reply suggester. The shared
// single-shot pipeline (validate -> load prompt -> call Claude -> parse/validate
// -> log ai_requests) lives in Claude.run.
type generator interface {
	// Feature is the ai_requests.feature value and the prompt directory name.
	Feature() string
	// PromptVersion selects prompts/{feature}/{version}.txt.
	PromptVersion() string
	// ValidateInput returns an *InputError when required input is missing.
	ValidateInput() error
	// BuildUserMessage returns the object serialized as the Claude user message.
	BuildUserMessage() (any, error)
	// ValidateOutput returns an *OutputError when the parsed JSON is invalid.
	ValidateOutput(data map[string]any) error
}

// Claude is the Anthropic-backed runner shared by the AI endpoints. It mirrors
// BaseAiService: one Claude call per invocation, one ai_requests row logged per
// call (success or failure). Construct it once and reuse it.
type Claude struct {
	DB      *gorm.DB
	APIKey  string
	BaseURL string
}

// NewClaude builds a Claude runner. BaseURL is overridable so tests can point
// the SDK at an httptest fake.
func NewClaude(db *gorm.DB, apiKey, baseURL string) *Claude {
	return &Claude{DB: db, APIKey: apiKey, BaseURL: baseURL}
}

// run executes the shared pipeline for a generator and always logs exactly one
// ai_requests row via a deferred writer keyed off the named return err (mirrors
// BaseAiService#call's ensure block). Tokens and payloads are captured only
// after a successful Claude call, so an ApiError leaves them nil while an
// OutputError still records them (the call succeeded; parsing failed) — matching
// Rails, where @input_tokens/@request_payload are set inside invoke_claude.
func (c *Claude) run(ctx context.Context, gen generator, userID *int64) (data map[string]any, err error) {
	started := time.Now()
	var inputTokens, outputTokens *int
	var requestPayload, responsePayload *string
	defer func() {
		c.writeAiRequest(gen, started, userID, inputTokens, outputTokens, requestPayload, responsePayload, err)
	}()

	if err = gen.ValidateInput(); err != nil {
		return nil, err
	}

	prompt, perr := prompts.Load(gen.Feature(), gen.PromptVersion())
	if perr != nil {
		err = &ApiError{Msg: perr.Error()}
		return nil, err
	}

	message, berr := gen.BuildUserMessage()
	if berr != nil {
		err = &ApiError{Msg: fmt.Sprintf("Unexpected error building message: %s", berr.Error())}
		return nil, err
	}
	messageJSON, merr := json.Marshal(message)
	if merr != nil {
		err = &ApiError{Msg: fmt.Sprintf("Unexpected error encoding message: %s", merr.Error())}
		return nil, err
	}

	text, in, out, cerr := c.invoke(ctx, prompt, string(messageJSON))
	if cerr != nil {
		err = cerr
		return nil, err
	}

	// The call succeeded — capture tokens and payloads before parsing, so an
	// OutputError below still logs them (parity with Rails).
	inputTokens, outputTokens = &in, &out
	rp := string(messageJSON)
	requestPayload, responsePayload = &rp, &text

	data, err = parseAndValidate(text, gen)
	return data, err
}

// invoke calls the Anthropic Messages API and returns the first text block plus
// token usage. Any provider/transport failure, or a non-text response, becomes
// an *ApiError (mirrors invoke_claude's rescue => Ai::ApiError). Retries are
// disabled so a 529 surfaces immediately as a 503 in tests.
func (c *Claude) invoke(ctx context.Context, systemPrompt, userMessage string) (text string, inputTokens, outputTokens int, err error) {
	client := anthropic.NewClient(
		option.WithAPIKey(c.APIKey),
		option.WithBaseURL(c.BaseURL),
		option.WithMaxRetries(0),
	)

	msg, apiErr := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(DefaultModel),
		MaxTokens: DefaultMaxTokens,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
		},
	})
	if apiErr != nil {
		return "", 0, 0, &ApiError{Msg: fmt.Sprintf("Claude API error: %s", apiErr.Error())}
	}

	if len(msg.Content) == 0 || msg.Content[0].Type != "text" {
		return "", 0, 0, &ApiError{Msg: fmt.Sprintf("Claude did not return text content (stop_reason: %s)", msg.StopReason)}
	}

	return msg.Content[0].Text, int(msg.Usage.InputTokens), int(msg.Usage.OutputTokens), nil
}

// parseAndValidate strips an optional ```json fence, parses the JSON object, and
// runs the generator's output-schema check. Parse failures and schema failures
// both become *OutputError (mirrors parse_and_validate).
func parseAndValidate(raw string, gen generator) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(stripCodeFence(raw)), &data); err != nil {
		return nil, &OutputError{Msg: fmt.Sprintf("Claude returned invalid JSON: %s", err.Error())}
	}
	if err := gen.ValidateOutput(data); err != nil {
		return nil, err
	}
	return data, nil
}

// requireNonEmptyString is the hand-rolled output-schema check shared by the
// generators: the key must be present, a string, and non-empty (len>0), else an
// *OutputError. The two Rails schemas are exactly this shape, so no jsonschema
// library is needed.
func requireNonEmptyString(data map[string]any, key string) error {
	v, ok := data[key]
	if !ok {
		return &OutputError{Msg: fmt.Sprintf("Output schema validation failed: missing %q", key)}
	}
	s, ok := v.(string)
	if !ok {
		return &OutputError{Msg: fmt.Sprintf("Output schema validation failed: %q must be a string", key)}
	}
	if len(s) == 0 {
		return &OutputError{Msg: fmt.Sprintf("Output schema validation failed: %q must be non-empty", key)}
	}
	return nil
}

var (
	leadingFence  = regexp.MustCompile("\\A```[a-zA-Z0-9]*[ \\t]*\\r?\\n?")
	trailingFence = regexp.MustCompile("\\r?\\n?```\\z")
)

// stripCodeFence removes a single leading/trailing ```json fence Claude
// sometimes adds despite being asked for raw JSON, mirroring
// BaseAiService#strip_code_fence. Already-clean JSON is returned untouched.
func stripCodeFence(text string) string {
	stripped := strings.TrimSpace(text)
	if !strings.HasPrefix(stripped, "```") {
		return stripped
	}
	stripped = leadingFence.ReplaceAllString(stripped, "")
	stripped = trailingFence.ReplaceAllString(stripped, "")
	return stripped
}

// writeAiRequest logs one ai_requests row per call, mirroring
// Ai::RequestLogging#write_ai_request. Best-effort: a logging failure is logged
// but never surfaced (parity with the Ruby rescue).
func (c *Claude) writeAiRequest(gen generator, started time.Time, userID *int64, inputTokens, outputTokens *int, requestPayload, responsePayload *string, callErr error) {
	if c.DB == nil {
		return
	}
	latency := int(time.Since(started).Milliseconds())
	version := gen.PromptVersion()
	record := &models.AiRequest{
		Feature:          gen.Feature(),
		Model:            DefaultModel,
		PromptVersion:    &version,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		LatencyMs:        &latency,
		EstimatedCostUSD: CostFor(DefaultModel, inputTokens, outputTokens),
		Success:          callErr == nil,
		RequestPayload:   requestPayload,
		ResponsePayload:  responsePayload,
		UserID:           userID,
	}
	if callErr != nil {
		msg := callErr.Error()
		record.ErrorMessage = &msg
	}
	if err := c.DB.Create(record).Error; err != nil {
		log.Printf("failed to write ai_request: %v", err)
	}
}
