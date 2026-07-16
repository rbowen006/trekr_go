// Package prompts embeds the Claude system prompts copied from rv_marketplace
// (see `make sync-contract`). These are contract artifacts — the exact prompt
// text Rails ships in app/prompts/{feature}/{version}.txt — embedded at build
// time the same way knowledge/regions.yml is (ADR-0013, internal/region), so the
// Go and Rails backends send byte-identical system prompts to Anthropic.
package prompts

import (
	"embed"
	"fmt"
)

//go:embed description_generator/v1.txt chat_reply/v1.txt
var files embed.FS

// Load returns the system prompt for a feature/version pair, mirroring
// BaseAiService#load_prompt reading app/prompts/{feature}/{version}.txt. A
// missing prompt returns an error the caller maps to an Ai::ApiError (matching
// Rails' Errno::ENOENT => ApiError).
func Load(feature, version string) (string, error) {
	data, err := files.ReadFile(fmt.Sprintf("%s/%s.txt", feature, version))
	if err != nil {
		return "", fmt.Errorf("prompt file not found: %s/%s.txt", feature, version)
	}
	return string(data), nil
}
