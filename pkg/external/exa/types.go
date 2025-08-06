package exa

import "time"

// SearchRequest represents a search request to Exa API
type SearchRequest struct {
	Query              string   `json:"query"`
	NumResults         int      `json:"numResults,omitempty"`
	Type               string   `json:"type,omitempty"` // "neural", "keyword", "auto"
	UseAutoprompt      bool     `json:"useAutoprompt,omitempty"`
	IncludeDomains     []string `json:"includeDomains,omitempty"`
	ExcludeDomains     []string `json:"excludeDomains,omitempty"`
	StartCrawlDate     string   `json:"startCrawlDate,omitempty"`
	EndCrawlDate       string   `json:"endCrawlDate,omitempty"`
	StartPublishedDate string   `json:"startPublishedDate,omitempty"`
	EndPublishedDate   string   `json:"endPublishedDate,omitempty"`
	Category           string   `json:"category,omitempty"`
	Contents           Contents `json:"contents,omitempty"`
}

// Contents specifies what content to include in results
type Contents struct {
	Text      bool `json:"text,omitempty"`
	Highlight bool `json:"highlight,omitempty"`
	Summary   bool `json:"summary,omitempty"`
}

// SearchResponse represents the response from Exa search
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	AutoPrompt string         `json:"autopromptString,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID              string    `json:"id"`
	URL             string    `json:"url"`
	Title           string    `json:"title"`
	Score           float64   `json:"score"`
	PublishedDate   string    `json:"publishedDate,omitempty"`
	Author          string    `json:"author,omitempty"`
	Text            string    `json:"text,omitempty"`
	Highlights      []string  `json:"highlights,omitempty"`
	HighlightScores []float64 `json:"highlightScores,omitempty"`
	Summary         string    `json:"summary,omitempty"`
}

// ContentsResponse represents the response from contents endpoint
type ContentsResponse struct {
	Results []ContentResult `json:"results"`
}

// ContentResult represents full content for a URL
type ContentResult struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	Extract       string `json:"extract"`
	Text          string `json:"text"`
	Author        string `json:"author,omitempty"`
	PublishedDate string `json:"publishedDate,omitempty"`
}

// CachedSearch represents a cached search in the database
type CachedSearch struct {
	ID             string
	Query          string
	QueryEmbedding []float32
	SearchType     string
	Results        []SearchResult
	CreatedAt      time.Time
	LastAccessed   time.Time
	AccessCount    int
	TTLDays        int
	ProjectContext string
}

// Feedback represents user feedback on search usefulness
type Feedback struct {
	SearchID        string
	UsefulnessScore float32
	FeedbackType    string // "implicit" or "explicit"
	UsefulURLs      []string
	CreatedAt       time.Time
}

