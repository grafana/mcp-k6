package docs

import (
	"testing"

	k6docslib "github.com/grafana/k6-docs-lib"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

func TestReadContentReadsMarkdownFromCache(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	require.NoError(t, afs.MkdirAll("/cache/markdown/using-k6", 0o755))
	require.NoError(t, fsext.WriteFile(afs, "/cache/markdown/using-k6/_index.md", []byte("# docs"), 0o600))

	provider := NewFromMultiIndex(nil, "/cache", afs)
	section := &k6docslib.Section{
		Slug:    "using-k6",
		RelPath: "using-k6/_index.md",
	}

	content, err := provider.ReadContent(section, "vtest")
	require.NoError(t, err)
	require.Equal(t, "# docs", string(content))
}

func TestReadContentRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	require.NoError(t, afs.MkdirAll("/cache/markdown", 0o755))
	require.NoError(t, fsext.WriteFile(afs, "/cache/secret.txt", []byte("secret"), 0o600))

	provider := NewFromMultiIndex(nil, "/cache", afs)
	section := &k6docslib.Section{
		Slug:    "escape",
		RelPath: "../secret.txt",
	}

	_, err := provider.ReadContent(section, "vtest")
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidRelPath)
}
