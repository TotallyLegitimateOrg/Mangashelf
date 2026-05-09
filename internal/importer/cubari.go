package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type CubariProvider struct{}

type cubariConfig struct {
	URL string `json:"url"`
}

type cubariRawChapter struct {
	Title       string                     `json:"title"`
	Volume      json.RawMessage            `json:"volume"`
	Groups      map[string]json.RawMessage `json:"groups"`
	LastUpdated cubariTimestamp            `json:"last_updated"`
}

type cubariRawSource struct {
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Artist      string                      `json:"artist"`
	Author      string                      `json:"author"`
	Cover       string                      `json:"cover"`
	Chapters    map[string]cubariRawChapter `json:"chapters"`
}

type cubariTimestamp string

func (CubariProvider) Name() string {
	return "cubari"
}

func (CubariProvider) ParseConfig(raw json.RawMessage) (json.RawMessage, error) {
	raw = normalizeRawJSON(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("config is required")
	}

	var config cubariConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("invalid cubari config: %w", err)
	}
	config.URL = strings.TrimSpace(config.URL)
	if config.URL == "" {
		return nil, fmt.Errorf("cubari config.url is required")
	}

	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p CubariProvider) FetchSource(ctx context.Context, config json.RawMessage) (*NormalizedSource, error) {
	parsed, err := p.parseConfig(config)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch cubari source: %d", resp.StatusCode)
	}

	var payload cubariRawSource
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid cubari JSON format: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("invalid cubari JSON format: %w", err)
	}

	chapters := make([]NormalizedChapter, 0)
	for chapterNumber, chapter := range payload.Chapters {
		number, err := parseCubariChapterNumber(chapterNumber)
		if err != nil {
			return nil, fmt.Errorf("invalid cubari chapter number %q: %w", chapterNumber, err)
		}

		volume, err := parseCubariVolume(chapter.Volume)
		if err != nil {
			return nil, fmt.Errorf("invalid cubari chapter volume for chapter %q: %w", chapterNumber, err)
		}

		updatedAt, err := parseCubariTimestamp(string(chapter.LastUpdated))
		if err != nil {
			return nil, fmt.Errorf("invalid cubari timestamp for chapter %q: %w", chapterNumber, err)
		}
		for groupName, rawPages := range chapter.Groups {
			pages, err := parseCubariGroupValue(rawPages)
			if err != nil {
				return nil, fmt.Errorf("invalid cubari group value for chapter %q group %q: %w", chapterNumber, groupName, err)
			}
			if len(pages) == 0 {
				continue
			}

			version := normalizeCubariVersion(groupName)
			langCode := "EN"
			chapters = append(chapters, NormalizedChapter{
				Identity:     CreateChapterIdentityKey(number, langCode, version),
				ChapNum:      number,
				Title:        strings.TrimSpace(chapter.Title),
				Volume:       volume,
				Version:      version,
				LangCode:     langCode,
				PublishDate:  &updatedAt,
				CreationDate: &updatedAt,
				LastUpdated:  updatedAt,
				Pages:        pages,
			})
		}
	}

	slices.SortFunc(chapters, func(a, b NormalizedChapter) int {
		if a.ChapNum < b.ChapNum {
			return -1
		}
		if a.ChapNum > b.ChapNum {
			return 1
		}
		return strings.Compare(a.Version, b.Version)
	})

	return &NormalizedSource{
		Provider:    p.Name(),
		Description: strings.TrimSpace(payload.Description),
		Artist:      strings.TrimSpace(payload.Artist),
		Author:      strings.TrimSpace(payload.Author),
		Cover:       strings.TrimSpace(payload.Cover),
		ExternalRef: parsed.URL,
		Config:      config,
		Chapters:    chapters,
	}, nil
}

func (p CubariProvider) parseConfig(raw json.RawMessage) (*cubariConfig, error) {
	canonical, err := p.ParseConfig(raw)
	if err != nil {
		return nil, err
	}

	var config cubariConfig
	if err := json.Unmarshal(canonical, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *cubariTimestamp) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*t = ""
		return nil
	}

	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = cubariTimestamp(value)
		return nil
	}

	var value json.Number
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("unsupported cubari timestamp: %w", err)
	}

	*t = cubariTimestamp(value.String())
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON content")
		}
		return err
	}
	return nil
}

func parseCubariChapterNumber(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("chapter number is empty")
	}

	number, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}

	return number, nil
}

func parseCubariVolume(raw json.RawMessage) (*float64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}

	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}

		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil
		}

		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, nil
		}
		return &number, nil
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return &number, nil
	}

	return nil, fmt.Errorf("volume must be a string, number, or empty")
}

func parseCubariGroupValue(raw json.RawMessage) ([]string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}

	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}

		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil
		}
		return []string{value}, nil
	}

	if raw[0] != '[' {
		return nil, fmt.Errorf("group value must be a string or array of strings")
	}

	var pages []string
	if err := json.Unmarshal(raw, &pages); err != nil {
		return nil, err
	}

	normalized := make([]string, 0, len(pages))
	for _, page := range pages {
		page = strings.TrimSpace(page)
		if page != "" {
			normalized = append(normalized, page)
		}
	}

	return normalized, nil
}

func normalizeCubariVersion(groupName string) string {
	version := strings.TrimSpace(groupName)
	if version == "" {
		return "Default"
	}
	return version
}

func parseCubariTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now().UTC(), nil
	}

	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}

	return time.Time{}, fmt.Errorf("expected unix timestamp")
}

func normalizeRawJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, value); err != nil {
		return json.RawMessage(bytes.TrimSpace(value))
	}
	return json.RawMessage(buf.Bytes())
}
