//go:build cgo
// +build cgo

// Copyright 2024 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package highlight

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/gohugoio/hugo/common/hugio"
	"github.com/gohugoio/hugo/markup/converter/hooks"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/cue"
	"github.com/smacker/go-tree-sitter/dockerfile"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/elm"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/groovy"
	"github.com/smacker/go-tree-sitter/hcl"
	tshtml "github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/ocaml"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/protobuf"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/sql"
	"github.com/smacker/go-tree-sitter/svelte"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/toml"
	"github.com/smacker/go-tree-sitter/yaml"
)

// treeSitterLanguages maps language names to their Tree-sitter grammar functions
var treeSitterLanguages = map[string]func() *sitter.Language{
	"bash":       bash.GetLanguage,
	"sh":         bash.GetLanguage,
	"shell":      bash.GetLanguage,
	"c":          c.GetLanguage,
	"cpp":        cpp.GetLanguage,
	"c++":        cpp.GetLanguage,
	"cxx":        cpp.GetLanguage,
	"cc":         cpp.GetLanguage,
	"csharp":     csharp.GetLanguage,
	"c#":         csharp.GetLanguage,
	"cs":         csharp.GetLanguage,
	"css":        css.GetLanguage,
	"cue":        cue.GetLanguage,
	"dockerfile": dockerfile.GetLanguage,
	"docker":     dockerfile.GetLanguage,
	"elixir":     elixir.GetLanguage,
	"ex":         elixir.GetLanguage,
	"exs":        elixir.GetLanguage,
	"elm":        elm.GetLanguage,
	"go":         golang.GetLanguage,
	"golang":     golang.GetLanguage,
	"groovy":     groovy.GetLanguage,
	"hcl":        hcl.GetLanguage,
	"tf":         hcl.GetLanguage,
	"html":       tshtml.GetLanguage,
	"htm":        tshtml.GetLanguage,
	"java":       java.GetLanguage,
	"javascript": javascript.GetLanguage,
	"js":         javascript.GetLanguage,
	"jsx":        javascript.GetLanguage,
	"kotlin":     kotlin.GetLanguage,
	"kt":         kotlin.GetLanguage,
	"lua":        lua.GetLanguage,
	"ocaml":      ocaml.GetLanguage,
	"ml":         ocaml.GetLanguage,
	"php":        php.GetLanguage,
	"protobuf":   protobuf.GetLanguage,
	"proto":      protobuf.GetLanguage,
	"python":     python.GetLanguage,
	"py":         python.GetLanguage,
	"ruby":       ruby.GetLanguage,
	"rb":         ruby.GetLanguage,
	"rust":       rust.GetLanguage,
	"rs":         rust.GetLanguage,
	"scala":      scala.GetLanguage,
	"sql":        sql.GetLanguage,
	"svelte":     svelte.GetLanguage,
	"swift":      swift.GetLanguage,
	"toml":       toml.GetLanguage,
	"yaml":       yaml.GetLanguage,
	"yml":        yaml.GetLanguage,
}

// treeSitterHighlighter uses Tree-sitter for syntax highlighting with fallback to Chroma
type treeSitterHighlighter struct {
	cfg            Config
	chromaFallback Highlighter
}

// NewWithTreeSitter creates a new highlighter that uses Tree-sitter when available,
// falling back to Chroma for unsupported languages
func NewWithTreeSitter(cfg Config) Highlighter {
	return &treeSitterHighlighter{
		cfg:            cfg,
		chromaFallback: chromaHighlighter{cfg: cfg},
	}
}

func (h *treeSitterHighlighter) Highlight(code, lang string, opts any) (string, error) {
	// Try Tree-sitter first
	if result, ok := h.tryTreeSitter(code, lang, opts); ok {
		return result, nil
	}

	// Fallback to Chroma
	return h.chromaFallback.Highlight(code, lang, opts)
}

func (h *treeSitterHighlighter) HighlightCodeBlock(ctx hooks.CodeblockContext, opts any) (HighlightResult, error) {
	// Try Tree-sitter first
	if result, ok := h.tryTreeSitterCodeBlock(ctx, opts); ok {
		return result, nil
	}

	// Fallback to Chroma
	return h.chromaFallback.HighlightCodeBlock(ctx, opts)
}

func (h *treeSitterHighlighter) RenderCodeblock(cctx context.Context, w hugio.FlexiWriter, ctx hooks.CodeblockContext) error {
	// Try Tree-sitter first
	if ok := h.tryRenderTreeSitterCodeblock(cctx, w, ctx); ok {
		return nil
	}

	// Fallback to Chroma
	return h.chromaFallback.RenderCodeblock(cctx, w, ctx)
}

func (h *treeSitterHighlighter) IsDefaultCodeBlockRenderer() bool {
	return true
}

// tryTreeSitter attempts to highlight code using Tree-sitter
func (h *treeSitterHighlighter) tryTreeSitter(code, lang string, opts any) (string, bool) {
	cfg := h.cfg
	if err := applyOptions(opts, &cfg); err != nil {
		return "", false
	}

	langFunc, supported := treeSitterLanguages[strings.ToLower(lang)]
	if !supported {
		return "", false
	}

	parser := sitter.NewParser()
	parser.SetLanguage(langFunc())

	tree, err := parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		return "", false
	}
	defer tree.Close()

	var result strings.Builder
	h.renderTreeSitterNode(tree.RootNode(), []byte(code), &result, cfg, lang)

	return result.String(), true
}

// tryTreeSitterCodeBlock attempts to highlight a code block using Tree-sitter
func (h *treeSitterHighlighter) tryTreeSitterCodeBlock(ctx hooks.CodeblockContext, opts any) (HighlightResult, bool) {
	cfg := h.cfg

	if err := applyOptionsFromMap(ctx.Options(), &cfg); err != nil {
		return HighlightResult{}, false
	}

	if err := applyOptions(opts, &cfg); err != nil {
		return HighlightResult{}, false
	}

	if err := applyOptionsFromCodeBlockContext(ctx, &cfg); err != nil {
		return HighlightResult{}, false
	}

	langFunc, supported := treeSitterLanguages[strings.ToLower(ctx.Type())]
	if !supported {
		return HighlightResult{}, false
	}

	parser := sitter.NewParser()
	parser.SetLanguage(langFunc())

	tree, err := parser.ParseCtx(context.Background(), nil, []byte(ctx.Inner()))
	if err != nil {
		return HighlightResult{}, false
	}
	defer tree.Close()

	var result strings.Builder
	h.renderTreeSitterNode(tree.RootNode(), []byte(ctx.Inner()), &result, cfg, ctx.Type())

	highlighted := result.String()
	return HighlightResult{
		highlighted: template.HTML(highlighted),
		innerLow:    0,
		innerHigh:   len(highlighted),
	}, true
}

// tryRenderTreeSitterCodeblock attempts to render a code block using Tree-sitter
func (h *treeSitterHighlighter) tryRenderTreeSitterCodeblock(cctx context.Context, w hugio.FlexiWriter, ctx hooks.CodeblockContext) bool {
	cfg := h.cfg

	if err := applyOptionsFromMap(ctx.Options(), &cfg); err != nil {
		return false
	}

	if err := applyOptionsFromCodeBlockContext(ctx, &cfg); err != nil {
		return false
	}

	langFunc, supported := treeSitterLanguages[strings.ToLower(ctx.Type())]
	if !supported {
		return false
	}

	parser := sitter.NewParser()
	parser.SetLanguage(langFunc())

	tree, err := parser.ParseCtx(cctx, nil, []byte(ctx.Inner()))
	if err != nil {
		return false
	}
	defer tree.Close()

	attributes := ctx.(hooks.AttributesOptionsSliceProvider).AttributesSlice()

	if !cfg.Hl_inline {
		writeDivStart(w, attributes, cfg.WrapperClass)
	}

	if cfg.Hl_inline {
		w.WriteString(fmt.Sprintf(`<code%s>`, inlineCodeAttrs(ctx.Type())))
	} else {
		WritePreStart(w, ctx.Type(), "")
	}

	h.renderTreeSitterNode(tree.RootNode(), []byte(ctx.Inner()), w, cfg, ctx.Type())

	if cfg.Hl_inline {
		w.WriteString("</code>")
	} else {
		w.WriteString(preEnd)
		writeDivEnd(w)
	}

	return true
}

// renderTreeSitterNode renders a Tree-sitter node with syntax highlighting
func (h *treeSitterHighlighter) renderTreeSitterNode(node *sitter.Node, source []byte, w interface{ WriteString(string) (int, error) }, cfg Config, lang string) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// Handle anonymous nodes (like punctuation) differently
	if nodeType == "" {
		// Anonymous node - just render content without styling
		content := node.Content(source)
		w.WriteString(html.EscapeString(content))
		return
	}

	// If this is a leaf node, render it with appropriate styling
	if node.ChildCount() == 0 {
		content := node.Content(source)
		class := h.mapNodeTypeToClass(nodeType)

		if class != "" {
			w.WriteString(fmt.Sprintf(`<span class="%s">`, class))
			w.WriteString(html.EscapeString(content))
			w.WriteString("</span>")
		} else {
			w.WriteString(html.EscapeString(content))
		}
		return
	}

	// For non-leaf nodes, handle special cases
	switch nodeType {
	case "string_literal", "string", "interpreted_string_literal", "raw_string_literal":
		// For string nodes, render the entire content as a string
		content := node.Content(source)
		w.WriteString(fmt.Sprintf(`<span class="s">%s</span>`, html.EscapeString(content)))
		return
	case "comment", "line_comment", "block_comment":
		// For comment nodes, render the entire content as a comment
		content := node.Content(source)
		w.WriteString(fmt.Sprintf(`<span class="c">%s</span>`, html.EscapeString(content)))
		return
	}

	// For other non-leaf nodes, check if we should style the whole node
	class := h.mapNodeTypeToClass(nodeType)
	if class != "" {
		w.WriteString(fmt.Sprintf(`<span class="%s">`, class))
	}

	// Recursively render children
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child != nil {
			h.renderTreeSitterNode(child, source, w, cfg, lang)
		}
	}

	// Close the span if we opened one
	if class != "" {
		w.WriteString("</span>")
	}
}

// mapNodeTypeToClass maps Tree-sitter node types to CSS classes
func (h *treeSitterHighlighter) mapNodeTypeToClass(nodeType string) string {
	// Map common Tree-sitter node types to Chroma-compatible CSS classes
	classMap := map[string]string{
		// Comments
		"comment":       "c",
		"line_comment":  "c1",
		"block_comment": "cm",

		// Strings
		"string":          "s",
		"string_literal":  "s",
		"raw_string":      "s",
		"template_string": "s",
		"char_literal":    "s1",

		// Numbers
		"number":  "m",
		"integer": "mi",
		"float":   "mf",
		"decimal": "m",

		// Keywords
		"keyword":   "k",
		"if":        "k",
		"else":      "k",
		"for":       "k",
		"while":     "k",
		"function":  "nf",
		"return":    "k",
		"import":    "kn",
		"from":      "kn",
		"class":     "k",
		"def":       "k",
		"var":       "k",
		"let":       "k",
		"const":     "k",
		"true":      "kc",
		"false":     "kc",
		"null":      "kc",
		"undefined": "kc",

		// Identifiers
		"identifier":           "n",
		"variable":             "n",
		"property":             "n",
		"field":                "n",
		"method":               "nf",
		"function_name":        "nf",
		"function_declaration": "nf",
		"function_definition":  "nf",

		// Types
		"type":            "kt",
		"type_identifier": "kt",
		"primitive_type":  "kt",

		// Operators
		"operator":        "o",
		"assignment":      "o",
		"binary_operator": "o",
		"unary_operator":  "o",

		// Punctuation
		"punctuation": "p",
		";":           "p",
		",":           "p",
		".":           "p",
		":":           "p",
		"(":           "p",
		")":           "p",
		"{":           "p",
		"}":           "p",
		"[":           "p",
		"]":           "p",

		// Attributes/Annotations
		"attribute":  "nd",
		"annotation": "nd",
		"decorator":  "nd",

		// Preprocessor
		"preproc":      "cp",
		"preprocessor": "cp",

		// Errors
		"ERROR": "err",
	}

	if class, exists := classMap[nodeType]; exists {
		return class
	}

	// Handle some common patterns
	if strings.Contains(nodeType, "comment") {
		return "c"
	}
	if strings.Contains(nodeType, "string") {
		return "s"
	}
	if strings.Contains(nodeType, "number") || strings.Contains(nodeType, "literal") {
		return "m"
	}
	if strings.Contains(nodeType, "keyword") {
		return "k"
	}
	if strings.Contains(nodeType, "type") {
		return "kt"
	}
	if strings.Contains(nodeType, "function") && !strings.Contains(nodeType, "call") {
		return "nf"
	}

	return ""
}
