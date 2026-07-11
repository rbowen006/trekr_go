// Package knowledge embeds the static trip-planning reference corpus copied
// from rv_marketplace (see `make sync-contract`). regions.yml is the single
// source of truth for the region vocabulary (ADR-0013) and is embedded at
// build time, mirroring Rails reading it from app/knowledge/regions.yml.
package knowledge

import _ "embed"

// RegionsYAML is the raw contents of regions.yml, parsed by internal/region.
//
//go:embed regions.yml
var RegionsYAML []byte
