package search

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
)

// Indexer is the interface that wraps the IndexDirectory method.
//
// It is used to index a directory of documents into a storage solution.
type Indexer interface {
	IndexDirectory(docsPath string) (int, error)
}

// SQLiteIndexer is an implementation of the Indexer interface that uses SQLite
// as the storage solution.
//
// It is used to index a directory of documents into a SQLite database.
type SQLiteIndexer struct {
	db *sql.DB
}

// NewSQLiteIndexer creates a new SQLiteIndexer with the given SQLite database.
func NewSQLiteIndexer(db *sql.DB) *SQLiteIndexer {
	return &SQLiteIndexer{db: db}
}

// IndexDirectory walks the provided docsPath and indexes all .md files it finds.
// It returns the number of files successfully indexed.
func (i *SQLiteIndexer) IndexDirectory(docsPath string) (int, error) {
	ctx := context.Background()
	count := 0
	err := filepath.WalkDir(docsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			chunks, perr := ParseMarkdown(path)
			if perr != nil {
				// Skip file on parse error, continue with other files
				//nolint:nilerr // Skip unparseable files, don't fail entire indexing
				return nil
			}
			for _, c := range chunks {
				if ierr := i.insertChunk(ctx, c); ierr != nil {
					return ierr
				}
			}
			count++
		}
		return nil
	})
	if err != nil {
		return count, err
	}
	return count, nil
}

func (i *SQLiteIndexer) insertChunk(ctx context.Context, c Result) error {
	_, err := i.db.ExecContext(ctx, `INSERT INTO documentation (title, content, path) VALUES (?, ?, ?)`,
		c.Title, c.Content, c.Path)
	return err
}
