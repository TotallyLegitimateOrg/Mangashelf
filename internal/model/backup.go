package model

import "encoding/json"

const BackupSchemaVersion = 2

type BackupManifest struct {
	Format        string            `json:"format"`
	SchemaVersion int               `json:"schemaVersion"`
	CreatedAt     string            `json:"createdAt"`
	App           BackupManifestApp `json:"app"`
	Counts        BackupCounts      `json:"counts"`
}

type BackupManifestApp struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"builtAt"`
}

type BackupCounts struct {
	Manga            int `json:"manga"`
	Chapters         int `json:"chapters"`
	ChapterSources   int `json:"chapterSources"`
	Collections      int `json:"collections"`
	DiscoverSections int `json:"discoverSections"`
}

type Backup struct {
	SchemaVersion    int                     `json:"schemaVersion"`
	Manga            []BackupManga           `json:"manga"`
	Chapters         []BackupChapter         `json:"chapters"`
	Collections      []BackupCollection      `json:"collections"`
	DiscoverSections []BackupDiscoverSection `json:"discoverSections"`
	ChapterSources   []BackupChapterSource   `json:"chapterSources"`
}

type BackupRestoreResult struct {
	MangaCount           int `json:"mangaCount"`
	ChapterCount         int `json:"chapterCount"`
	CollectionCount      int `json:"collectionCount"`
	DiscoverSectionCount int `json:"discoverSectionCount"`
	ChapterSourceCount   int `json:"chapterSourceCount"`
}

type BackupManga struct {
	ID              string      `json:"id"`
	PrimaryTitle    string      `json:"primaryTitle"`
	SecondaryTitles []string    `json:"secondaryTitles"`
	Synopsis        string      `json:"synopsis"`
	ThumbnailURL    string      `json:"thumbnailUrl"`
	BannerURL       string      `json:"bannerUrl"`
	ContentRating   string      `json:"contentRating"`
	Status          string      `json:"status"`
	Artist          string      `json:"artist"`
	Author          string      `json:"author"`
	Rating          *float64    `json:"rating"`
	ShareURL        string      `json:"shareUrl"`
	ArtworkURLs     []string    `json:"artworkUrls"`
	TagGroups       []TagGroup  `json:"tagGroups"`
	AdditionalInfo  []InfoEntry `json:"additionalInfo"`
	CreatedAt       string      `json:"createdAt"`
	UpdatedAt       string      `json:"updatedAt"`
}

type BackupChapter struct {
	ID             string        `json:"id"`
	MangaID        string        `json:"mangaId"`
	LangCode       string        `json:"langCode"`
	ChapNum        float64       `json:"chapNum"`
	Title          string        `json:"title"`
	Version        string        `json:"version"`
	Volume         *float64      `json:"volume"`
	PublishDate    *string       `json:"publishDate"`
	CreationDate   *string       `json:"creationDate"`
	SortingIndex   *float64      `json:"sortingIndex"`
	AdditionalInfo []InfoEntry   `json:"additionalInfo"`
	Pages          []string      `json:"pages"`
	LastUpdated    string        `json:"lastUpdated"`
	Origin         ChapterOrigin `json:"origin"`
}

type BackupCollection struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	SortOrder int      `json:"sortOrder"`
	MangaIDs  []string `json:"mangaIds"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

type BackupDiscoverSection struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Subtitle  string                `json:"subtitle"`
	Type      string                `json:"type"`
	Mode      string                `json:"mode"`
	LiveRule  *DiscoverLiveRule     `json:"liveRule,omitempty"`
	SortOrder int                   `json:"sortOrder"`
	Items     []DiscoverSectionItem `json:"items"`
}

type BackupChapterSource struct {
	ID                  string          `json:"id"`
	MangaID             string          `json:"mangaId"`
	Provider            string          `json:"provider"`
	Mode                string          `json:"mode"`
	Config              json.RawMessage `json:"config"`
	SyncIntervalMinutes int             `json:"syncIntervalMinutes"`
	CreatedAt           string          `json:"createdAt"`
	UpdatedAt           string          `json:"updatedAt"`
}
