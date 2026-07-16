package ai

import (
	"context"
	"strings"
)

// DescriptionInput carries the listing facts a caller supplies to generate a
// description. Pointer fields distinguish "omitted" from a zero value, matching
// Rails' blank? checks on the corresponding params.
type DescriptionInput struct {
	RvType      string
	Town        string
	State       string
	MaxGuests   *int
	PetFriendly bool
	PricePerDay *float64
}

// GenerateDescription runs the description-generator feature for the given
// input, logging one ai_requests row attributed to userID. Mirrors
// Ai::DescriptionGenerator.call.
func (c *Claude) GenerateDescription(ctx context.Context, in DescriptionInput, userID *int64) (map[string]any, error) {
	return c.run(ctx, &descriptionGenerator{in: in}, userID)
}

// descriptionGenerator is the per-feature seam for listing descriptions
// (Ai::DescriptionGenerator).
type descriptionGenerator struct {
	in DescriptionInput
}

func (g *descriptionGenerator) Feature() string       { return "description_generator" }
func (g *descriptionGenerator) PromptVersion() string { return "v1" }

// ValidateInput enforces REQUIRED_FIELDS (rv_type, town, state, max_guests).
// The string fields are blank when empty; max_guests is blank when omitted
// (mirrors Integer#blank? being false for any present value, including 0).
func (g *descriptionGenerator) ValidateInput() error {
	var missing []string
	if strings.TrimSpace(g.in.RvType) == "" {
		missing = append(missing, "rv_type")
	}
	if strings.TrimSpace(g.in.Town) == "" {
		missing = append(missing, "town")
	}
	if strings.TrimSpace(g.in.State) == "" {
		missing = append(missing, "state")
	}
	if g.in.MaxGuests == nil {
		missing = append(missing, "max_guests")
	}
	if len(missing) > 0 {
		return &InputError{Msg: "Missing required fields: " + strings.Join(missing, ", ")}
	}
	return nil
}

// descriptionUserMessage fixes the field order of the user message JSON to match
// the Ruby hash in DescriptionGenerator#build_user_message.
type descriptionUserMessage struct {
	RvType      string   `json:"rv_type"`
	Town        string   `json:"town"`
	State       string   `json:"state"`
	MaxGuests   *int     `json:"max_guests"`
	PetFriendly bool     `json:"pet_friendly"`
	PricePerDay *float64 `json:"price_per_day"`
}

func (g *descriptionGenerator) BuildUserMessage() (any, error) {
	return descriptionUserMessage{
		RvType:      g.in.RvType,
		Town:        g.in.Town,
		State:       g.in.State,
		MaxGuests:   g.in.MaxGuests,
		PetFriendly: g.in.PetFriendly,
		PricePerDay: g.in.PricePerDay,
	}, nil
}

// ValidateOutput requires a non-empty "description" string (the trivial
// hand-rolled equivalent of the Rails JSON schema).
func (g *descriptionGenerator) ValidateOutput(data map[string]any) error {
	return requireNonEmptyString(data, "description")
}
