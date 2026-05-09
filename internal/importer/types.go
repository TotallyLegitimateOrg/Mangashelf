package importer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ChapterIdentity struct {
	Key string `json:"key"`
}

type NormalizedChapter struct {
	Identity     ChapterIdentity
	ChapNum      float64
	Title        string
	Volume       *float64
	Version      string
	LangCode     string
	PublishDate  *time.Time
	CreationDate *time.Time
	LastUpdated  time.Time
	Pages        []string
}

type NormalizedSource struct {
	Provider    string
	Description string
	Artist      string
	Author      string
	Cover       string
	ExternalRef string
	Config      json.RawMessage
	Chapters    []NormalizedChapter
}

type Provider interface {
	Name() string
	ParseConfig(raw json.RawMessage) (json.RawMessage, error)
	FetchSource(ctx context.Context, config json.RawMessage) (*NormalizedSource, error)
}

var providers = map[string]Provider{
	"cubari": CubariProvider{},
}

func ResolveProvider(name string) (Provider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	provider, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
	return provider, nil
}

func CreateChapterIdentityKey(chapNum float64, langCode string, version string) ChapterIdentity {
	return ChapterIdentity{
		Key: fmt.Sprintf("%v::%s::%s", chapNum, strings.ToUpper(strings.TrimSpace(langCode)), strings.TrimSpace(version)),
	}
}

const proxyIDPrefix = "proxy_"

func CreateProxyChapterID(provider string, sourceID string, identity ChapterIdentity) string {
	payload, _ := json.Marshal(map[string]string{
		"provider":   strings.ToLower(strings.TrimSpace(provider)),
		"sourceId":   sourceID,
		"chapterKey": identity.Key,
	})
	return proxyIDPrefix + base64.RawURLEncoding.EncodeToString(payload)
}

func ParseProxyChapterID(chapterID string) (provider string, sourceID string, chapterKey string, ok bool) {
	if !strings.HasPrefix(chapterID, proxyIDPrefix) {
		return "", "", "", false
	}

	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(chapterID, proxyIDPrefix))
	if err != nil {
		return "", "", "", false
	}

	var payload struct {
		Provider   string `json:"provider"`
		SourceID   string `json:"sourceId"`
		ChapterKey string `json:"chapterKey"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", "", "", false
	}

	if payload.Provider == "" || payload.SourceID == "" || payload.ChapterKey == "" {
		return "", "", "", false
	}

	return payload.Provider, payload.SourceID, payload.ChapterKey, true
}
