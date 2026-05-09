package model

import "encoding/json"

type InfoEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Tag struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type TagGroup struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Tags  []Tag  `json:"tags"`
}

type SearchFilter struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

type DiscoverSearchQuery struct {
	Title   string         `json:"title"`
	Filters []SearchFilter `json:"filters"`
}

type DiscoverLiveRule struct {
	Preset string `json:"preset"`
	Limit  int    `json:"limit"`
}

type Manga struct {
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
	ChapterCount    int         `json:"chapterCount"`
	CreatedAt       string      `json:"createdAt"`
	UpdatedAt       string      `json:"updatedAt"`
}

type MangaSearchOptions struct {
	Query         string
	ContentRating []string
	Status        []string
	Tags          []string
	MinRating     *float64
	MaxRating     *float64
	Sort          string
}

type MangaPayload struct {
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
}

type Collection struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	SortOrder  int    `json:"sortOrder"`
	MangaCount int    `json:"mangaCount"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type CollectionPayload struct {
	Title string `json:"title"`
}

type CollectionMangaPayload struct {
	MangaIDs []string `json:"mangaIds"`
}

type CollectionChangesPayload struct {
	Additions []string `json:"additions"`
	Deletions []string `json:"deletions"`
}

type ChapterOrigin struct {
	Kind         string  `json:"kind"`
	Mode         string  `json:"mode"`
	ReadOnly     bool    `json:"readOnly"`
	Provider     *string `json:"provider"`
	SourceID     *string `json:"sourceId"`
	SourceStatus *string `json:"sourceStatus"`
	ChapterKey   *string `json:"chapterKey"`
}

type ChapterListItem struct {
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
	PageCount      int           `json:"pageCount"`
	LastUpdated    string        `json:"lastUpdated"`
	Origin         ChapterOrigin `json:"origin"`
}

type ChapterDetail struct {
	ChapterListItem
	Pages []string `json:"pages"`
}

type ChapterPayload struct {
	LangCode       string      `json:"langCode"`
	ChapNum        float64     `json:"chapNum"`
	Title          string      `json:"title"`
	Version        string      `json:"version"`
	Volume         *float64    `json:"volume"`
	PublishDate    *string     `json:"publishDate"`
	CreationDate   *string     `json:"creationDate"`
	SortingIndex   *float64    `json:"sortingIndex"`
	AdditionalInfo []InfoEntry `json:"additionalInfo"`
	Pages          []string    `json:"pages"`
}

type ChapterSource struct {
	ID                   string          `json:"id"`
	MangaID              string          `json:"mangaId"`
	Provider             string          `json:"provider"`
	Mode                 string          `json:"mode"`
	Config               json.RawMessage `json:"config"`
	Status               string          `json:"status"`
	LastError            string          `json:"lastError"`
	LastSeenChapterCount *int            `json:"lastSeenChapterCount"`
	SyncIntervalMinutes  int             `json:"syncIntervalMinutes"`
	LastSyncedAt         *string         `json:"lastSyncedAt"`
	CreatedAt            string          `json:"createdAt"`
	UpdatedAt            string          `json:"updatedAt"`
}

type ChapterSourceSyncLog struct {
	ID             string `json:"id"`
	SourceID       string `json:"sourceId"`
	MangaID        string `json:"mangaId"`
	Status         string `json:"status"`
	InsertedCount  int    `json:"insertedCount"`
	UpdatedCount   int    `json:"updatedCount"`
	UnchangedCount int    `json:"unchangedCount"`
	SkippedCount   int    `json:"skippedCount"`
	Error          string `json:"error"`
	CreatedAt      string `json:"createdAt"`
}

type ChapterImportPayload struct {
	Provider            string          `json:"provider"`
	Mode                string          `json:"mode"`
	Config              json.RawMessage `json:"config"`
	SyncIntervalMinutes int             `json:"syncIntervalMinutes"`
}

type ChapterImportResult struct {
	Mode           string          `json:"mode"`
	InsertedCount  int             `json:"insertedCount"`
	UpdatedCount   int             `json:"updatedCount"`
	UnchangedCount int             `json:"unchangedCount"`
	SkippedCount   int             `json:"skippedCount"`
	Source         *ChapterSource  `json:"source,omitempty"`
	Chapters       []ChapterDetail `json:"chapters"`
}

type DiscoverSectionItem struct {
	ID            string               `json:"id"`
	Type          string               `json:"type"`
	MangaID       string               `json:"mangaId,omitempty"`
	ChapterID     string               `json:"chapterId,omitempty"`
	ChapNum       *float64             `json:"chapNum,omitempty"`
	ImageURL      string               `json:"imageUrl,omitempty"`
	Title         string               `json:"title,omitempty"`
	Subtitle      string               `json:"subtitle,omitempty"`
	Supertitle    string               `json:"supertitle,omitempty"`
	Name          string               `json:"name,omitempty"`
	PublishDate   *string              `json:"publishDate"`
	ContentRating *string              `json:"contentRating"`
	Metadata      json.RawMessage      `json:"metadata,omitempty"`
	SearchQuery   *DiscoverSearchQuery `json:"searchQuery,omitempty"`
}

type DiscoverSection struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Subtitle  string                `json:"subtitle"`
	Type      string                `json:"type"`
	SortOrder int                   `json:"sortOrder"`
	Items     []DiscoverSectionItem `json:"items"`
}

type DiscoverSectionConfig struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Subtitle  string                `json:"subtitle"`
	Type      string                `json:"type"`
	Mode      string                `json:"mode"`
	LiveRule  *DiscoverLiveRule     `json:"liveRule,omitempty"`
	SortOrder int                   `json:"sortOrder"`
	Items     []DiscoverSectionItem `json:"items"`
}

type DiscoverSectionPayload struct {
	Title    string                `json:"title"`
	Subtitle string                `json:"subtitle"`
	Type     string                `json:"type"`
	Mode     string                `json:"mode"`
	LiveRule *DiscoverLiveRule     `json:"liveRule,omitempty"`
	Items    []DiscoverSectionItem `json:"items"`
}
