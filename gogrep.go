package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	Version   = "N/A"
	BuildTime = "N/A"
)

func main() {
	duration := flag.Duration("timeout", 500*time.Millisecond, "overall search timeout (Go duration, e.g. 5s, 2m)")
	flag.Usage = func() {
		fmt.Printf("%s by Brian Ketelsen\n", os.Args[0])
		fmt.Printf("Version %s, Built: %s \n", Version, BuildTime)
		fmt.Println("Usage:")
		fmt.Printf("\tgogrep [flags] path pattern \n")
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(-1)
	}
	path := flag.Arg(0)
	pattern := flag.Arg(1)

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	matches, err := search(ctx, path, pattern)
	if err != nil {
		log.Fatal(err)
	}
	for _, name := range matches {
		fmt.Println(name)
	}
	fmt.Println(len(matches), "hits")
}

// search walks root concurrently and returns paths to regular .go files
// whose contents contain pattern. The walk and the per-file scans run in
// parallel and respect ctx for cancellation. Ordering of the returned slice
// is unspecified.
func search(ctx context.Context, root, pattern string) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx)

	paths := make(chan string, 100)

	g.Go(func() error {
		defer close(paths)
		return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			select {
			case paths <- p:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	})

	results := make(chan string, 100)

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	needle := []byte(pattern)
	for i := 0; i < workers; i++ {
		g.Go(func() error {
			for p := range paths {
				if err := ctx.Err(); err != nil {
					return err
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				if !bytes.Contains(data, needle) {
					continue
				}
				select {
				case results <- p:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	go func() {
		_ = g.Wait()
		close(results)
	}()

	var out []string
	for r := range results {
		out = append(out, r)
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}
