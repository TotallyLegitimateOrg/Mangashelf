package model

import "testing"

func TestValidateDiscoverPayloadRequiresAtLeastOneItem(t *testing.T) {
	err := ValidateDiscoverPayload(DiscoverSectionPayload{
		Title:    "Featured",
		Subtitle: "",
		Type:     "simpleCarousel",
		Mode:     "manual",
		Items:    []DiscoverSectionItem{},
	})
	if err == nil {
		t.Fatal("ValidateDiscoverPayload returned nil, want error")
	}
	if got := err.Error(); got != "at least one item is required" {
		t.Fatalf("ValidateDiscoverPayload error = %q, want %q", got, "at least one item is required")
	}
}

func TestValidateDiscoverPayloadRequiresLiveRuleForLiveSections(t *testing.T) {
	err := ValidateDiscoverPayload(DiscoverSectionPayload{
		Title: "Featured",
		Type:  "simpleCarousel",
		Mode:  "live",
	})
	if err == nil {
		t.Fatal("ValidateDiscoverPayload returned nil, want error")
	}
	if got := err.Error(); got != "live sections require liveRule" {
		t.Fatalf("ValidateDiscoverPayload error = %q, want %q", got, "live sections require liveRule")
	}
}

func TestValidateDiscoverPayloadRejectsInvalidLivePreset(t *testing.T) {
	err := ValidateDiscoverPayload(DiscoverSectionPayload{
		Title: "Featured",
		Type:  "chapterUpdates",
		Mode:  "live",
		LiveRule: &DiscoverLiveRule{
			Preset: "title_asc",
			Limit:  10,
		},
	})
	if err == nil {
		t.Fatal("ValidateDiscoverPayload returned nil, want error")
	}
	if got := err.Error(); got != `invalid liveRule preset "title_asc" for section "chapterUpdates"` {
		t.Fatalf("ValidateDiscoverPayload error = %q, want %q", got, `invalid liveRule preset "title_asc" for section "chapterUpdates"`)
	}
}

func TestValidateDiscoverPayloadRejectsNonPositiveLiveLimit(t *testing.T) {
	err := ValidateDiscoverPayload(DiscoverSectionPayload{
		Title: "Featured",
		Type:  "simpleCarousel",
		Mode:  "live",
		LiveRule: &DiscoverLiveRule{
			Preset: "title_asc",
			Limit:  0,
		},
	})
	if err == nil {
		t.Fatal("ValidateDiscoverPayload returned nil, want error")
	}
	if got := err.Error(); got != "live sections require liveRule.limit to be greater than zero" {
		t.Fatalf("ValidateDiscoverPayload error = %q, want %q", got, "live sections require liveRule.limit to be greater than zero")
	}
}
