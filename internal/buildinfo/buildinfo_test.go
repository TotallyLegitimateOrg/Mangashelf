package buildinfo

import "testing"

func TestNormalizeValue(t *testing.T) {
	t.Parallel()

	if got := normalizeValue("  value  ", "fallback"); got != "value" {
		t.Fatalf("expected trimmed value, got %q", got)
	}
	if got := normalizeValue("", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for empty value, got %q", got)
	}
	if got := normalizeValue("(devel)", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for devel value, got %q", got)
	}
}
