package models

import "time"

// AiRequest maps to Rails' ai_requests table: one row logged per AI/embedding
// call (success or failure), mirroring BaseAiService/Embedder's ensure block.
// Nullable columns are pointers so an omitted value persists as SQL NULL.
type AiRequest struct {
	ID               int64     `gorm:"primaryKey"`
	Feature          string    `gorm:"column:feature"`
	Model            string    `gorm:"column:model"`
	PromptVersion    *string   `gorm:"column:prompt_version"`
	InputTokens      *int      `gorm:"column:input_tokens"`
	OutputTokens     *int      `gorm:"column:output_tokens"`
	LatencyMs        *int      `gorm:"column:latency_ms"`
	EstimatedCostUSD *float64  `gorm:"column:estimated_cost_usd"`
	Success          bool      `gorm:"column:success"`
	ErrorMessage     *string   `gorm:"column:error_message"`
	RequestPayload   *string   `gorm:"column:request_payload"`
	ResponsePayload  *string   `gorm:"column:response_payload"`
	UserID           *int64    `gorm:"column:user_id"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (AiRequest) TableName() string { return "ai_requests" }
