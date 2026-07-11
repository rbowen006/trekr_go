package region_test

import (
	"strings"
	"testing"

	"github.com/rbowen/trekr_go/internal/region"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Resolver behaviour mirrors rv_marketplace spec/models/region/resolver_spec.rb:
// a town in a covered region resolves to that region's slug; anything else -> "".
// state and postcode are accepted (parity with Region::Resolver.call) but the
// current Rails resolver matches on exact town name only.
func TestManifest_Resolve(t *testing.T) {
	m, err := region.Load()
	require.NoError(t, err)

	cases := []struct {
		name     string
		town     string
		state    string
		postcode string
		want     string
	}{
		{"covered town resolves to slug", "Lorne", "VIC", "3232", "great-ocean-road"},
		{"second town of a region", "Apollo Bay", "VIC", "", "great-ocean-road"},
		{"second town resolves (Bondi -> sydney)", "Bondi Beach", "NSW", "", "sydney"},
		{"single-town region", "Katoomba", "NSW", "", "blue-mountains"},
		{"qld region", "Port Douglas", "QLD", "", "tropical-north-queensland"},
		{"town outside every region", "Gosford", "NSW", "2250", ""},
		{"empty town", "", "", "", ""},
		{"case-sensitive, no match", "lorne", "VIC", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, m.Resolve(tc.town, tc.state, tc.postcode))
		})
	}
}

// Mirrors region_spec.rb '.find'.
func TestManifest_Find(t *testing.T) {
	m, err := region.Load()
	require.NoError(t, err)

	r, ok := m.Find("great-ocean-road")
	require.True(t, ok)
	assert.Equal(t, "Great Ocean Road", r.Name)
	assert.Equal(t, "VIC", r.State)

	_, ok = m.Find("atlantis")
	assert.False(t, ok)
}

// Mirrors region_spec.rb '.all (manifest invariants)': every town in the real
// manifest maps to exactly one region.
func TestLoad_RealManifestTownsAreUnique(t *testing.T) {
	m, err := region.Load()
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, r := range m.All() {
		for _, town := range r.Towns {
			assert.False(t, seen[town], "town %q appears in more than one region", town)
			seen[town] = true
		}
	}
}

// Mirrors region_spec.rb: a town mapped to two regions raises a ManifestError
// naming the offending town(s). Parse enforces the invariant (ADR-0013) at the
// single load point, so a duplicate can never resolve silently by file order.
func TestParse_DuplicateTownIsRejected(t *testing.T) {
	yaml := `
- slug: a
  name: A
  state: NSW
  towns: [Gosford]
- slug: b
  name: B
  state: NSW
  towns: [Gosford]
`
	_, err := region.Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Gosford")
}

func TestParse_EmptyManifest(t *testing.T) {
	m, err := region.Parse([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, m.All())
	assert.Equal(t, "", m.Resolve("Lorne", "VIC", ""))
}

func TestParse_Malformed(t *testing.T) {
	_, err := region.Parse([]byte("\tnot: valid: yaml:"))
	require.Error(t, err)
}

func TestMustLoad_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { _ = region.MustLoad() })
	assert.True(t, strings.HasPrefix(region.MustLoad().All()[0].Slug, "great-ocean"))
}
