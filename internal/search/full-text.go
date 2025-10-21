// Package search provides k6 documentation search functionality.
package search

import (
	"context"
	"database/sql"
	"strings"
)

// FullTextSearch implements full-text search using SQLite FTS5.
type FullTextSearch struct {
	db *sql.DB
}

var _ Search = &FullTextSearch{}

// NewFullTextSearcher creates a new FullTextSearch instance.
func NewFullTextSearcher(db *sql.DB) *FullTextSearch {
	return &FullTextSearch{db: db}
}

// Search returns up to limit results for the provided MATCH query.
func (s *FullTextSearch) Search(ctx context.Context, query string, opts Options) ([]Result, error) {
	// Preprocess the query to handle multi-word searches
	processedQuery := preprocessQuery(query)

	rows, err := s.db.QueryContext(ctx, `
        SELECT title, content, path
        FROM documentation
        WHERE documentation MATCH ?
        ORDER BY bm25(documentation, ?, ?, ?)
        LIMIT ?`, processedQuery, BM25WeightTitle, BM25WeightContent, BM25WeightPath, opts.MaxResults)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			// Log the error, but don't override the primary error
			_ = closeErr
		}
	}()

	var results []Result
	for rows.Next() {
		var c Result
		if err := rows.Scan(&c.Title, &c.Content, &c.Path); err != nil {
			return nil, err
		}
		results = append(results, c)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// preprocessQuery converts space-separated words to FTS5 AND queries
// while preserving explicit FTS5 syntax like AND, OR, quotes, etc.
func preprocessQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return query
	}

	// If query already contains FTS5 operators or quotes, return as-is
	if strings.Contains(query, " AND ") || strings.Contains(query, " OR ") ||
		strings.Contains(query, " NEAR ") || strings.Contains(query, "\"") ||
		strings.Contains(query, "*") || strings.Contains(query, "(") {
		return query
	}

	// Split on spaces and join with AND
	words := strings.Fields(query)
	if len(words) <= 1 {
		return query
	}

	return strings.Join(words, " AND ")
}
