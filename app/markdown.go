package app

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
	),
)

var mdPolicy = bluemonday.UGCPolicy()

func renderMarkdown(input string) template.HTML {
	if strings.TrimSpace(input) == "" {
		return template.HTML("")
	}
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(input), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(input))
	}
	safe := mdPolicy.SanitizeBytes(buf.Bytes())
	return template.HTML(safe)
}
