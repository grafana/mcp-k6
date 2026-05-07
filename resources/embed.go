package resources

import "embed"

//go:embed *.md
var resourceFiles embed.FS
