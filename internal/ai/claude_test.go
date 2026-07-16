package ai

import "testing"

func TestStripCodeFence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain json untouched", `{"description":"hi"}`, `{"description":"hi"}`},
		{"json fence", "```json\n{\"description\":\"hi\"}\n```", `{"description":"hi"}`},
		{"bare fence", "```\n{\"reply\":\"ok\"}\n```", `{"reply":"ok"}`},
		{"surrounding whitespace", "  \n{\"reply\":\"ok\"}\n  ", `{"reply":"ok"}`},
		{"fence with trailing spaces after lang", "```json  \n{\"a\":1}\n```", `{"a":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripCodeFence(c.in); got != c.want {
				t.Fatalf("stripCodeFence(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 500, "…"); got != "hello" {
		t.Fatalf("short string should be untouched, got %q", got)
	}
	// Longer than the limit: keep (limit - len(omission)) runes, then omission.
	if got := truncate("hello", 3, "…"); got != "he…" {
		t.Fatalf("truncate(hello,3,…) = %q, want %q", got, "he…")
	}
	// Rune-counted, not byte-counted: a 4-rune multibyte string within the limit.
	s := "café"
	if got := truncate(s, 4, "…"); got != s {
		t.Fatalf("multibyte within limit should be untouched, got %q", got)
	}
}

func TestCostFor(t *testing.T) {
	in, out := 120, 30
	cost := CostFor("claude-sonnet-4-6", &in, &out)
	if cost == nil {
		t.Fatal("expected a cost for a known model")
	}
	want := 120*0.000003 + 30*0.000015
	if *cost != want {
		t.Fatalf("CostFor = %v, want %v", *cost, want)
	}

	if CostFor("unknown-model", &in, &out) != nil {
		t.Fatal("unknown model should yield nil cost")
	}
	if CostFor("claude-sonnet-4-6", nil, &out) != nil {
		t.Fatal("nil input tokens should yield nil cost")
	}
}
