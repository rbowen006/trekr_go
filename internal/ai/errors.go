package ai

// The three AI error categories mirror rv_marketplace's Ai::InputError /
// Ai::ApiError / Ai::OutputError (app/services/ai/*_error.rb). Handlers branch
// on them with errors.As to map to HTTP status: InputError->400 (fail),
// ApiError->503 (error), OutputError->500 (error). Concrete pointer types keep
// the mapping explicit and let errors.As unwrap them from wrapped errors.

// InputError signals invalid or missing caller input (Ai::InputError). Maps to
// HTTP 400 with a JSend "fail" body.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

// ApiError signals an upstream/provider failure — Claude API error, missing
// prompt, or an unexpected internal error (Ai::ApiError). Maps to HTTP 503.
type ApiError struct{ Msg string }

func (e *ApiError) Error() string { return e.Msg }

// OutputError signals that Claude's response failed parsing or output-schema
// validation (Ai::OutputError). Maps to HTTP 500.
type OutputError struct{ Msg string }

func (e *OutputError) Error() string { return e.Msg }
