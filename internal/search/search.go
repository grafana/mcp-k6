package search

import "context"

// Search is the interface that wraps the Search method.
//
// It is used to search for documents in the index.
type Search interface {
	Search(ctx context.Context, query string, options Options) ([]Result, error)
}

// Result is the result of a search query.
//
// It contains the title, content, and path of the document.
// The path is the relative path to the document from the root of the index.
// The content is the content of the document.
// The title is the title of the document.
type Result struct {
	// The title of the document.
	Title string `json:"title"`

	// The content of the document.
	Content string `json:"content"`

	// The path of the document. Relative to the index.
	Path string `json:"path"`

	// Metadata (optional) is a map of key-value pairs containing additional information related to the document.
	Metadata map[string]string `json:"metadata"`

	// Source (optional) is the original source of the result being returned
	Source string `json:"source"`

	// Rank represents the scoring or relevance of the document in the context of a search result.
	Rank float64 `json:"rank"`
}

// Options is the options for a search query.
//
// It contains the maximum number of results to return.
type Options struct {
	MaxResults int `json:"max_results"`
}

// DefaultOptions returns default search configuration.
func DefaultOptions() Options {
	return Options{
		MaxResults: defaultMaxResults,
	}
}

const (
	defaultMaxResults = 10
)

// BM25 weighting coefficients used to score FTS5 rows.
// These values prioritize matches in the title, then content, then path.
// Tune cautiously: large skews can drown out relevant content.
const (
	BM25WeightTitle   = 3.0
	BM25WeightContent = 1.0
	BM25WeightPath    = 0.1
)
