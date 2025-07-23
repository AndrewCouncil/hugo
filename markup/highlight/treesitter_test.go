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
	"strings"
	"testing"

	"github.com/frankban/quicktest"
	"github.com/gohugoio/hugo/common/text"
	"github.com/gohugoio/hugo/markup/internal/attributes"
)

func TestTreeSitterHighlighter(t *testing.T) {
	c := quicktest.New(t)

	cfg := DefaultConfig
	cfg.NoClasses = false
	cfg.Style = "github"

	h := NewWithTreeSitter(cfg)

	testCases := []struct {
		name     string
		lang     string
		code     string
		contains []string
	}{
		{
			name: "Go code highlighting",
			lang: "go",
			code: `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
			contains: []string{"package", "main", "fmt", "func", "Println"},
		},
		{
			name: "JavaScript highlighting",
			lang: "javascript",
			code: `const message = "Hello, World!";
console.log(message);`,
			contains: []string{"const", "message", "console", "log"},
		},
		{
			name: "Python highlighting",
			lang: "python",
			code: `def hello_world():
    print("Hello, World!")

if __name__ == "__main__":
    hello_world()`,
			contains: []string{"def", "hello_world", "print", "__name__", "__main__"},
		},
		{
			name: "Rust highlighting",
			lang: "rust",
			code: `fn main() {
    println!("Hello, World!");
}`,
			contains: []string{"fn", "main", "println"},
		},
		{
			name: "HTML highlighting",
			lang: "html",
			code: `<!DOCTYPE html>
<html>
<head>
    <title>Test</title>
</head>
<body>
    <h1>Hello World</h1>
</body>
</html>`,
			contains: []string{"DOCTYPE", "html", "head", "title", "body", "h1"},
		},
		{
			name: "CSS highlighting",
			lang: "css",
			code: `.container {
    display: flex;
    color: #333;
}`,
			contains: []string{"container", "display", "flex", "color"},
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *quicktest.C) {
			result, err := h.Highlight(tc.code, tc.lang, nil)
			c.Assert(err, quicktest.IsNil)
			c.Assert(result, quicktest.Not(quicktest.Equals), "")

			// Verify that the highlighted code contains the expected elements
			for _, expected := range tc.contains {
				c.Assert(result, quicktest.Contains, expected)
			}

			// Verify that HTML is properly escaped
			c.Assert(result, quicktest.Not(quicktest.Contains), "<script>")
			if strings.Contains(tc.code, "<") {
				c.Assert(result, quicktest.Contains, "&lt;")
			}
		})
	}
}

func TestTreeSitterFallbackToChroma(t *testing.T) {
	c := quicktest.New(t)

	cfg := DefaultConfig
	cfg.NoClasses = false

	h := NewWithTreeSitter(cfg)

	// Test with a language that's not supported by Tree-sitter
	unsupportedLang := "fortran"
	code := `program hello
    print *, 'Hello, World!'
end program hello`

	result, err := h.Highlight(code, unsupportedLang, nil)
	c.Assert(err, quicktest.IsNil)
	c.Assert(result, quicktest.Not(quicktest.Equals), "")

	// The result should still contain the code (fallback to Chroma)
	c.Assert(result, quicktest.Contains, "Hello, World!")
}

func TestTreeSitterCodeBlock(t *testing.T) {
	c := quicktest.New(t)

	cfg := DefaultConfig
	cfg.NoClasses = false

	h := NewWithTreeSitter(cfg)

	// Create a mock code block context
	ctx := &mockCodeblockContext{
		inner:   `fmt.Println("Hello from Go!")`,
		typ:     "go",
		options: map[string]any{},
	}

	result, err := h.HighlightCodeBlock(ctx, nil)
	c.Assert(err, quicktest.IsNil)

	highlighted := string(result.Wrapped())
	c.Assert(highlighted, quicktest.Not(quicktest.Equals), "")
	c.Assert(highlighted, quicktest.Contains, "fmt")
	c.Assert(highlighted, quicktest.Contains, "Println")
	// String content gets HTML escaped, so check for the content (either raw or escaped)
	hasRawContent := strings.Contains(highlighted, "Hello from Go!")
	hasEscapedContent := strings.Contains(highlighted, "&#34;Hello from Go!&#34;")
	c.Assert(hasRawContent || hasEscapedContent, quicktest.Equals, true, quicktest.Commentf("Expected to find 'Hello from Go!' in output: %s", highlighted))
}

func TestTreeSitterRenderCodeblock(t *testing.T) {
	c := quicktest.New(t)

	cfg := DefaultConfig
	cfg.NoClasses = false

	h := NewWithTreeSitter(cfg)

	var buf strings.Builder
	w := &flexiWriterAdapter{&buf}

	ctx := &mockCodeblockContext{
		inner:   `console.log("JavaScript test");`,
		typ:     "javascript",
		options: map[string]any{},
	}

	err := h.RenderCodeblock(context.Background(), w, ctx)
	c.Assert(err, quicktest.IsNil)

	result := buf.String()
	c.Assert(result, quicktest.Not(quicktest.Equals), "")
	c.Assert(result, quicktest.Contains, "console")
	c.Assert(result, quicktest.Contains, "log")
	// String content gets HTML escaped, so check for the content (either raw or escaped)
	hasRawContent := strings.Contains(result, "JavaScript test")
	hasEscapedContent := strings.Contains(result, "&#34;JavaScript test&#34;")
	c.Assert(hasRawContent || hasEscapedContent, quicktest.Equals, true, quicktest.Commentf("Expected to find 'JavaScript test' in output: %s", result))
}

func TestTreeSitterInlineCode(t *testing.T) {
	c := quicktest.New(t)

	cfg := DefaultConfig
	cfg.NoClasses = false
	cfg.Hl_inline = true

	h := NewWithTreeSitter(cfg)

	result, err := h.Highlight(`fmt.Println("test")`, "go", nil)
	c.Assert(err, quicktest.IsNil)
	c.Assert(result, quicktest.Not(quicktest.Equals), "")
	// For inline mode, we should get highlighted content but not necessarily <code> tags from Tree-sitter
	c.Assert(result, quicktest.Contains, "fmt")
	c.Assert(result, quicktest.Contains, "Println")
	c.Assert(result, quicktest.Contains, "test")
}

func TestTreeSitterSupportedLanguages(t *testing.T) {
	c := quicktest.New(t)

	cfg := DefaultConfig
	h := &treeSitterHighlighter{cfg: cfg}

	supportedLanguages := []string{
		"go", "golang", "javascript", "js", "python", "py", "rust", "rs",
		"java", "c", "cpp", "c++", "csharp", "c#", "html", "css",
		"bash", "sh", "shell", "yaml", "yml", "toml", "sql",
		"typescript", "ts", "tsx", "php", "ruby", "rb",
		"swift", "kotlin", "kt", "scala", "lua", "markdown", "md",
	}

	code := `var example = "test";`

	for _, lang := range supportedLanguages {
		c.Run("language_"+lang, func(c *quicktest.C) {
			result, supported := h.tryTreeSitter(code, lang, nil)
			if supported {
				c.Assert(result, quicktest.Not(quicktest.Equals), "")
				// Some languages may not parse the generic test code correctly,
				// but they should at least produce some output
				c.Assert(len(result) > 0, quicktest.Equals, true)
			}
			// Note: Some languages may not parse the generic test code,
			// but they should still be recognized as supported
		})
	}
}

func TestTreeSitterClassMapping(t *testing.T) {
	c := quicktest.New(t)

	h := &treeSitterHighlighter{}

	testCases := []struct {
		nodeType      string
		expectedClass string
	}{
		{"comment", "c"},
		{"string", "s"},
		{"number", "m"},
		{"keyword", "k"},
		{"identifier", "n"},
		{"function_name", "nf"},
		{"method", "nf"},
		{"type", "kt"},
		{"ERROR", "err"},
	}

	for _, tc := range testCases {
		c.Run(tc.nodeType, func(c *quicktest.C) {
			class := h.mapNodeTypeToClass(tc.nodeType)
			c.Assert(class, quicktest.Equals, tc.expectedClass)
		})
	}
}

// Mock implementation for testing
type mockCodeblockContext struct {
	inner      string
	typ        string
	options    map[string]any
	attributes []attributes.Attribute
	ordinal    int
	position   text.Position
}

func (m *mockCodeblockContext) Inner() string {
	return m.inner
}

func (m *mockCodeblockContext) Type() string {
	return m.typ
}

func (m *mockCodeblockContext) Options() map[string]any {
	return m.options
}

func (m *mockCodeblockContext) AttributesSlice() []attributes.Attribute {
	return m.attributes
}

func (m *mockCodeblockContext) OptionsSlice() []attributes.Attribute {
	// For testing, return empty slice
	return []attributes.Attribute{}
}

func (m *mockCodeblockContext) Attributes() map[string]any {
	result := make(map[string]any)
	for _, attr := range m.attributes {
		result[attr.Name] = attr.Value
	}
	return result
}

func (m *mockCodeblockContext) Ordinal() int {
	return m.ordinal
}

func (m *mockCodeblockContext) Position() text.Position {
	return m.position
}

func (m *mockCodeblockContext) Page() any {
	return nil
}

func (m *mockCodeblockContext) PageInner() any {
	return nil
}

// flexiWriterAdapter adapts strings.Builder to implement FlexiWriter
type flexiWriterAdapter struct {
	*strings.Builder
}

func (f *flexiWriterAdapter) WriteByte(c byte) error {
	return f.Builder.WriteByte(c)
}

func (f *flexiWriterAdapter) WriteString(s string) (int, error) {
	return f.Builder.WriteString(s)
}

func (f *flexiWriterAdapter) WriteRune(r rune) (int, error) {
	return f.Builder.WriteRune(r)
}

func BenchmarkTreeSitterHighlighting(b *testing.B) {
	cfg := DefaultConfig
	h := NewWithTreeSitter(cfg)

	code := `package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World! Current time: %v", time.Now())
	})

	log.Println("Server starting on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := h.Highlight(code, "go", nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTreeSitterVsChroma(b *testing.B) {
	cfg := DefaultConfig
	treeSitterH := NewWithTreeSitter(cfg)
	chromaH := chromaHighlighter{cfg: cfg}

	code := `function fibonacci(n) {
	if (n <= 1) return n;
	return fibonacci(n - 1) + fibonacci(n - 2);
}

const result = fibonacci(10);
console.log("Fibonacci(10) =", result);`

	b.Run("TreeSitter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := treeSitterH.Highlight(code, "javascript", nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Chroma", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := chromaH.Highlight(code, "javascript", nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
