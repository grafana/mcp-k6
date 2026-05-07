// Package internal provides shared constants and utilities for the k6 MCP server.
package internal

import "path/filepath"

const (
	// DistFolderName is the name of the dist folder where distribution files (generated
	// for instance by the documentation indexing, or the type definitions collection) are stored.
	DistFolderName = "dist"

	// DistDefinitionsFolderName is the name of the definitions folder where the type definitions are stored
	// within the dist folder.
	DistDefinitionsFolderName = "definitions"

	// DistTypesFolderName is the name of the types folder where the type definitions are stored
	DistTypesFolderName = "types"

	// DistK6FolderName is the name of the k6 folder where the type definitions are stored
	DistK6FolderName = "k6"

	// DistDTSFileSuffix is the file extension for the type definitions files
	DistDTSFileSuffix = ".d.ts"
)

// DefinitionsPath is the path to the definitions folder on disk (relative to the module root).
//
//nolint:gochecknoglobals // Computed constant path based on const values
var DefinitionsPath = filepath.Join(DistFolderName, DistDefinitionsFolderName, DistTypesFolderName, DistK6FolderName)

// EmbeddedDefinitionsPath is the path prefix within the dist.TypeDefinitions embedded FS.
//
//nolint:gochecknoglobals // Computed constant path based on const values
var EmbeddedDefinitionsPath = filepath.Join(DistDefinitionsFolderName, DistTypesFolderName, DistK6FolderName)
