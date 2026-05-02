package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func writeTree(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

func sortedRel(t *testing.T, base string, matches []string) []string {
	t.Helper()
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(base, m)
		if err != nil {
			t.Fatalf("rel %s: %v", m, err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func TestSearch(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		pattern string
		want    []string
	}{
		{
			name: "single match in go file",
			files: map[string]string{
				"a.go":  "package a\nfunc Foo() {}\n",
				"b.txt": "func Foo() {}\n",
			},
			pattern: "Foo",
			want:    []string{"a.go"},
		},
		{
			name: "no matches",
			files: map[string]string{
				"a.go": "package a\n",
				"b.go": "package b\n",
			},
			pattern: "ZZZ",
			want:    nil,
		},
		{
			name: "nested matches",
			files: map[string]string{
				"a.go":         "needle here",
				"sub/b.go":     "haystack",
				"sub/c.go":     "needle in sub",
				"sub/dir/d.go": "deep needle",
			},
			pattern: "needle",
			want: []string{
				"a.go",
				"sub/c.go",
				"sub/dir/d.go",
			},
		},
		{
			name: "non-go files ignored",
			files: map[string]string{
				"hit.go":     "MAGIC",
				"hit.txt":    "MAGIC",
				"hit.md":     "MAGIC",
				"go":         "MAGIC",
				"foo.go.bak": "MAGIC",
			},
			pattern: "MAGIC",
			want:    []string{"hit.go"},
		},
		{
			name: "substring match",
			files: map[string]string{
				"a.go": "function calls",
			},
			pattern: "func",
			want:    []string{"a.go"},
		},
		{
			name:    "empty tree",
			files:   map[string]string{},
			pattern: "anything",
			want:    nil,
		},
		{
			name: "case sensitive",
			files: map[string]string{
				"a.go": "Hello World",
			},
			pattern: "hello",
			want:    nil,
		},
		{
			name: "directory named like a go file is descended into",
			files: map[string]string{
				"weird.go/inside.go": "WANTED",
				"plain.go":           "WANTED",
			},
			pattern: "WANTED",
			want:    []string{"plain.go", "weird.go/inside.go"},
		},
		{
			name: "directory ending in .go is not itself matched",
			files: map[string]string{
				"foo.go/bar.txt": "needle",
			},
			pattern: "needle",
			want:    nil,
		},
		{
			name: "deeply nested tree",
			files: map[string]string{
				"a/b/c/d/e/f/g.go": "TOKEN",
				"a/b/c/d/e/f/h.go": "nope",
			},
			pattern: "TOKEN",
			want:    []string{"a/b/c/d/e/f/g.go"},
		},
		{
			name: "utf-8 multibyte pattern",
			files: map[string]string{
				"a.go": "// 日本語 comment\npackage a\n",
				"b.go": "// english only\n",
			},
			pattern: "日本語",
			want:    []string{"a.go"},
		},
		{
			name: "pattern spans newline",
			files: map[string]string{
				"a.go": "ab\ncd",
			},
			pattern: "b\nc",
			want:    []string{"a.go"},
		},
		{
			name: "pattern matches multiple files",
			files: map[string]string{
				"a.go":   "X",
				"b/c.go": "X",
				"d.go":   "Y",
			},
			pattern: "X",
			want:    []string{"a.go", "b/c.go"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			writeTree(t, base, tc.files)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := search(ctx, base, tc.pattern)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			rel := sortedRel(t, base, got)
			if !reflect.DeepEqual(rel, tc.want) {
				t.Fatalf("matches mismatch\n got: %v\nwant: %v", rel, tc.want)
			}
		})
	}
}

func TestSearchEmptyPatternMatchesEveryGoFile(t *testing.T) {
	base := t.TempDir()
	writeTree(t, base, map[string]string{
		"a.go":     "any content",
		"sub/b.go": "another",
		"c.txt":    "skipped",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := search(ctx, base, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	rel := sortedRel(t, base, got)
	want := []string{"a.go", "sub/b.go"}
	if !reflect.DeepEqual(rel, want) {
		t.Fatalf("matches mismatch\n got: %v\nwant: %v", rel, want)
	}
}

func TestSearchAlreadyCanceledContext(t *testing.T) {
	base := t.TempDir()
	writeTree(t, base, map[string]string{
		"a.go": "x",
		"b.go": "x",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := search(ctx, base, "x")
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchDeadlineExceeded(t *testing.T) {
	base := t.TempDir()
	writeTree(t, base, map[string]string{
		"a.go": "x",
	})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := search(ctx, base, "x")
	if err == nil {
		t.Fatal("expected error from expired deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchNonexistentRoot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := search(ctx, filepath.Join(t.TempDir(), "no-such-dir"), "x")
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestSearchSingleFileRoot(t *testing.T) {
	base := t.TempDir()
	writeTree(t, base, map[string]string{
		"only.go": "HELLO",
	})
	target := filepath.Join(base, "only.go")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := search(ctx, target, "HELLO")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 || got[0] != target {
		t.Fatalf("expected [%s], got %v", target, got)
	}
}
