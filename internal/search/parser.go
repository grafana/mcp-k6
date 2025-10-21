package search

import (
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ParseMarkdown parses a Markdown file and extracts content for indexing.
func ParseMarkdown(path string) ([]Result, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(src))

	var chunks []Result
	var currentTitle string
	var buffer strings.Builder

	flush := func() {
		if buffer.Len() == 0 {
			return
		}
		chunks = append(chunks, Result{
			Title:   currentTitle,
			Content: strings.TrimSpace(buffer.String()),
			Path:    path,
		})
		buffer.Reset()
	}

	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch node := n.(type) {
		case *ast.Heading:
			// Capture the first H1 as title, but include all headings in content
			if currentTitle == "" && node.Level == 1 {
				currentTitle = string(node.Text(src))
			}
			buffer.WriteString("\n" + string(node.Text(src)) + "\n")
		case *ast.Paragraph:
			buffer.WriteString("\n" + string(node.Text(src)) + "\n")
		case *ast.FencedCodeBlock:
			var code strings.Builder
			lines := node.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				code.Write(line.Value(src))
			}
			if code.Len() > 0 {
				buffer.WriteString("\n" + code.String() + "\n")
			}
		case *ast.CodeBlock:
			var code strings.Builder
			lines := node.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				code.Write(line.Value(src))
			}
			if code.Len() > 0 {
				buffer.WriteString("\n" + code.String() + "\n")
			}
		case *ast.CodeSpan:
			textBytes := node.Text(src)
			if len(textBytes) > 0 {
				buffer.WriteString(" `" + string(textBytes) + "` ")
			}
		case *ast.List:
			buffer.WriteString("\n" + string(node.Text(src)) + "\n")
		}

		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(doc)
	flush()
	return chunks, nil
}
