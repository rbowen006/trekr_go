// Package region resolves a listing's location to a trip-planning Region
// (ADR-0013). It is the pure-Go mirror of rv_marketplace's Region PORO and
// Region::Resolver: the vocabulary is static reference data read from
// knowledge/regions.yml, never CRUD'd at runtime. No HTTP, no DB.
package region

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rbowen/trekr_go/knowledge"
	"gopkg.in/yaml.v3"
)

// Region is a sub-state area with an authored knowledge corpus (ADR-0013).
// Fields mirror the manifest and Rails' Region attr_readers.
type Region struct {
	Slug  string   `yaml:"slug"`
	Name  string   `yaml:"name"`
	State string   `yaml:"state"`
	Towns []string `yaml:"towns"`
	Doc   string   `yaml:"doc"`
}

// Manifest is the loaded, validated region vocabulary. Region order is
// preserved from the file so Resolve matches by file order like Rails'
// `Region.all.find`.
type Manifest struct {
	regions []Region
}

// ManifestError signals that regions.yml violates a manifest invariant
// (ADR-0013). Mirrors Rails' Region::ManifestError, raised loudly at load so a
// duplicate town can never resolve silently by file order.
type ManifestError struct {
	Message string
}

func (e *ManifestError) Error() string { return e.Message }

// Parse reads a regions.yml body into a validated Manifest. It enforces the
// unique-towns invariant at this single load point (the one place every path —
// resolver, coverage gate, chunk loading — goes through in Rails).
func Parse(data []byte) (Manifest, error) {
	var regions []Region
	if err := yaml.Unmarshal(data, &regions); err != nil {
		return Manifest{}, fmt.Errorf("region: parsing manifest: %w", err)
	}
	if err := ensureUniqueTowns(regions); err != nil {
		return Manifest{}, err
	}
	return Manifest{regions: regions}, nil
}

// Load parses the embedded knowledge/regions.yml manifest.
func Load() (Manifest, error) {
	return Parse(knowledge.RegionsYAML)
}

// MustLoad returns the embedded manifest or panics if it is invalid. The
// embedded manifest is a compile-time artifact of the binary, so an invalid one
// is a build/deploy bug — this mirrors Rails raising on Region.all. Use in
// server wiring; use Load in tests that tolerate error.
func MustLoad() Manifest {
	m, err := Load()
	if err != nil {
		panic(err)
	}
	return m
}

// All returns the regions in manifest (file) order.
func (m Manifest) All() []Region { return m.regions }

// Find returns the region with the given slug. Mirrors Rails' Region.find,
// with an explicit found flag instead of nil.
func (m Manifest) Find(slug string) (Region, bool) {
	for _, r := range m.regions {
		if r.Slug == slug {
			return r, true
		}
	}
	return Region{}, false
}

// Resolve maps a listing's location to a Region slug, or "" if it falls outside
// every covered region. Mirrors Region::Resolver.call: state and postcode are
// accepted for parity but the current resolver matches on exact town name only,
// taking the first region (by file order) whose towns include it.
func (m Manifest) Resolve(town, state, postcode string) string {
	for _, r := range m.regions {
		for _, t := range r.Towns {
			if t == town {
				return r.Slug
			}
		}
	}
	return ""
}

// ensureUniqueTowns fails if any town is mapped to more than one region
// (ADR-0013). Mirrors Region.ensure_unique_towns!, listing offending towns in a
// stable order.
func ensureUniqueTowns(regions []Region) error {
	counts := map[string]int{}
	for _, r := range regions {
		for _, t := range r.Towns {
			counts[t]++
		}
	}
	var dupes []string
	for town, n := range counts {
		if n > 1 {
			dupes = append(dupes, town)
		}
	}
	if len(dupes) == 0 {
		return nil
	}
	sort.Strings(dupes)
	return &ManifestError{Message: fmt.Sprintf(
		"regions.yml maps %s to more than one region; "+
			"each town must resolve to exactly one region (ADR-0013).",
		strings.Join(dupes, ", "),
	)}
}
