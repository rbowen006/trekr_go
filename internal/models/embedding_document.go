package models

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// EmbeddingDocument composes the text embedded for semantic search (ADR-0011):
// structured fields rendered as prose, then the free text. Price is
// deliberately excluded. Mirrors RvListing#embedding_document exactly, including
// field order and the trailing strip.
func (l *RvListing) EmbeddingDocument() string {
	parts := []string{
		humanize(RvTypeName(l.RvType)) + " in " + l.Town + ", " + l.State + ".",
		// max_guests is NOT NULL, so Rails' `max_guests.present?` is always true.
		"Sleeps " + strconv.Itoa(l.MaxGuests) + " guests.",
	}
	if l.PetFriendly {
		parts = append(parts, "Pet-friendly.")
	}
	parts = append(parts, l.Title+".")
	parts = append(parts, l.Description)
	return strings.TrimSpace(strings.Join(parts, " "))
}

// ContentHash is the SHA256 of the composed document, used to skip re-embedding
// unchanged text (ListingEmbedding.content_hash_for, ADR-0011).
func ContentHash(document string) string {
	sum := sha256.Sum256([]byte(document))
	return hex.EncodeToString(sum[:])
}

// humanize mirrors ActiveSupport's String#humanize for our rv_type enum values:
// underscores become spaces and the first letter is capitalized
// (e.g. "camper_trailer" -> "Camper trailer").
func humanize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
