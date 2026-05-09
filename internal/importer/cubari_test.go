package importer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCubariProviderParseConfig(t *testing.T) {
	provider := CubariProvider{}

	config, err := provider.ParseConfig(json.RawMessage(`{"url":" https://example.com/source.json "}`))
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	if string(config) != `{"url":"https://example.com/source.json"}` {
		t.Fatalf("ParseConfig returned %s", config)
	}

	if _, err := provider.ParseConfig(json.RawMessage(`{"url":"   "}`)); err == nil {
		t.Fatalf("ParseConfig succeeded, want error for blank URL")
	}
}

func TestCubariProviderFetchSource(t *testing.T) {
	provider := CubariProvider{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"title":" Example ",
			"description":" Desc ",
			"artist":" Artist ",
			"author":" Author ",
			"cover":" https://example.com/cover.jpg ",
			"chapters":{
				"2":{
					"title":" Chapter 2 ",
					"volume":"2",
					"last_updated":1714435200,
					"groups":{"Group B":["https://example.com/ch2.jpg"]}
				},
				"1":{
					"title":" Chapter 1 ",
					"volume":1,
					"last_updated":"1714435100",
					"groups":{"":"https://example.com/ch1.jpg"}
				}
			}
		}`)
	}))
	defer server.Close()

	config, err := provider.ParseConfig(json.RawMessage(`{"url":"` + server.URL + `"}`))
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}

	source, err := provider.FetchSource(context.Background(), config)
	if err != nil {
		t.Fatalf("FetchSource returned error: %v", err)
	}

	if source.Provider != "cubari" {
		t.Fatalf("Provider = %q, want cubari", source.Provider)
	}
	if source.Description != "Desc" || source.Artist != "Artist" || source.Author != "Author" {
		t.Fatalf("unexpected source metadata: %+v", source)
	}
	if source.Cover != "https://example.com/cover.jpg" {
		t.Fatalf("Cover = %q, want cover URL", source.Cover)
	}
	if source.ExternalRef != server.URL {
		t.Fatalf("ExternalRef = %q, want %q", source.ExternalRef, server.URL)
	}
	if got := len(source.Chapters); got != 2 {
		t.Fatalf("len(Chapters) = %d, want 2", got)
	}

	first := source.Chapters[0]
	if first.Identity.Key != CreateChapterIdentityKey(1, "EN", "Default").Key {
		t.Fatalf("first identity key = %q", first.Identity.Key)
	}
	if first.LastUpdated != time.Unix(1714435100, 0).UTC() {
		t.Fatalf("first LastUpdated = %s", first.LastUpdated)
	}
	if got := first.Pages[0]; got != "https://example.com/ch1.jpg" {
		t.Fatalf("first page = %q, want ch1 page", got)
	}

	second := source.Chapters[1]
	if second.Identity.Key != CreateChapterIdentityKey(2, "EN", "Group B").Key {
		t.Fatalf("second identity key = %q", second.Identity.Key)
	}
}

func TestCubariProviderFetchSourceRejectsInvalidPayload(t *testing.T) {
	provider := CubariProvider{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"chapters":{"extra":{"groups":{"Default":["https://example.com/page.jpg"]}}}}`)
	}))
	defer server.Close()

	config, err := provider.ParseConfig(json.RawMessage(`{"url":"` + server.URL + `"}`))
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}

	if _, err := provider.FetchSource(context.Background(), config); err == nil {
		t.Fatalf("FetchSource succeeded, want error")
	}
}

func TestProxyChapterIDRoundTrip(t *testing.T) {
	identity := CreateChapterIdentityKey(12.5, "EN", "Group A")
	chapterID := CreateProxyChapterID("cubari", "source-1", identity)

	provider, sourceID, chapterKey, ok := ParseProxyChapterID(chapterID)
	if !ok {
		t.Fatalf("ParseProxyChapterID returned ok=false")
	}
	if provider != "cubari" || sourceID != "source-1" || chapterKey != identity.Key {
		t.Fatalf("unexpected proxy ID payload: %q %q %q", provider, sourceID, chapterKey)
	}
}
