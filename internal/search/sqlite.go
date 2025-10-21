package search

import (
	"database/sql"

	// Import sqlite3 driver for database/sql
	_ "github.com/mattn/go-sqlite3"
)

// InitSQLiteDB opens (or creates) the SQLite database at the given path and ensures
// the FTS5 table exists with the intended tokenizer options.
// If recreate is true, it drops any existing table named `documentation` first to rebuild.
func InitSQLiteDB(path string, recreate bool) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Optionally recreate the FTS5 table for documentation chunks.
	// Use unicode61 tokenizer with extra token characters useful for code.
	if recreate {
		if _, err := db.Exec(`DROP TABLE IF EXISTS documentation;`); err != nil {
			return nil, err
		}
	}
	_, err = db.Exec(`
        CREATE VIRTUAL TABLE IF NOT EXISTS documentation
        USING fts5(
            title,
            content,
            path,
            tokenize = 'unicode61 remove_diacritics 2 tokenchars ''_/:#@-$'''
        );
    `)
	if err != nil {
		return nil, err
	}
	return db, nil
}
