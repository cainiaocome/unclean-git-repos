// Command unclean-git-repos scans the immediate subdirectories (one level deep)
// of a root directory and reports which ones are git repositories with
// uncommitted changes (a "dirty" working tree).
//
// Usage:
//
//	unclean-git-repos [flags] [root]
//
// If root is omitted the current working directory is used.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
)

// ANSI color helpers. Colors are disabled automatically when stdout is not a
// terminal (see initColors) or when --no-color is passed.
var (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cGray   = "\033[90m"
	cCyan   = "\033[36m"
)

// version is set at build time via -ldflags "-X main.version=...". See Makefile.
var version = "dev"

func disableColors() {
	cReset, cBold, cRed, cGreen, cYellow, cGray, cCyan = "", "", "", "", "", "", ""
}

// repoResult holds the outcome of inspecting a single subdirectory.
type repoResult struct {
	name    string   // directory name (relative to root)
	isRepo  bool     // true if the directory is the root of a git repo
	dirty   bool     // true if the repo has uncommitted changes
	changes []string // porcelain status lines, only populated when dirty
	err     error    // non-nil if git inspection failed unexpectedly
}

func main() {
	verbose := flag.Bool("v", false, "also list clean repositories and skipped (non-git) folders")
	noColor := flag.Bool("no-color", false, "disable colored output")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [root]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintln(os.Stderr, "Finds git repositories (one level under root) that have uncommitted changes.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s\n", filepath.Base(os.Args[0]), version)
		return
	}

	if *noColor || os.Getenv("NO_COLOR") != "" || !isTerminal(os.Stdout) {
		disableColors()
	}

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot resolve %q: %v\n", root, err)
		os.Exit(2)
	}

	entries, err := os.ReadDir(absRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read %q: %v\n", absRoot, err)
		os.Exit(2)
	}

	// Inspect each subdirectory concurrently — git calls are I/O bound.
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []repoResult
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip the git internal directory that appears when root is itself a repo.
		if e.Name() == ".git" {
			continue
		}
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			r := inspect(filepath.Join(absRoot, name))
			r.name = name
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(e.Name())
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool { return results[i].name < results[j].name })

	report(absRoot, results, *verbose)

	// Exit non-zero when at least one dirty repo is found, so the tool is
	// usable in scripts and CI.
	for _, r := range results {
		if r.dirty {
			os.Exit(1)
		}
	}
}

// inspect determines whether dir is the root of a git repository and, if so,
// whether it has uncommitted changes.
func inspect(dir string) repoResult {
	// A subdirectory of a git repo would answer "true" to is-inside-work-tree,
	// so instead we ask git for the repository root and require it to match dir.
	// This correctly rejects plain folders that merely live inside a parent repo.
	top, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		// Not a git repository (or git refused) -> skip it.
		return repoResult{isRepo: false}
	}
	topAbs, _ := filepath.Abs(top)
	dirAbs, _ := filepath.Abs(dir)
	// Resolve symlinks so /var vs /private/var style differences don't trip us.
	if resolved, e := filepath.EvalSymlinks(topAbs); e == nil {
		topAbs = resolved
	}
	if resolved, e := filepath.EvalSymlinks(dirAbs); e == nil {
		dirAbs = resolved
	}
	if topAbs != dirAbs {
		// dir sits inside a repo but is not itself the repo root -> skip.
		return repoResult{isRepo: false}
	}

	out, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		return repoResult{isRepo: true, err: err}
	}
	if out == "" {
		return repoResult{isRepo: true, dirty: false}
	}

	var changes []string
	sc := bufio.NewScanner(bytes.NewReader([]byte(out)))
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			changes = append(changes, line)
		}
	}
	return repoResult{isRepo: true, dirty: true, changes: changes}
}

// runGit runs a git command inside dir and returns trimmed stdout.
func runGit(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return string(bytes.TrimRight(stdout.Bytes(), "\n")), nil
}

// report prints a human-friendly summary of the scan.
func report(root string, results []repoResult, verbose bool) {
	var dirty, clean, skipped, failed []repoResult
	for _, r := range results {
		switch {
		case r.err != nil:
			failed = append(failed, r)
		case !r.isRepo:
			skipped = append(skipped, r)
		case r.dirty:
			dirty = append(dirty, r)
		default:
			clean = append(clean, r)
		}
	}

	fmt.Printf("Scanning %s%s%s\n\n", cBold, root, cReset)

	if len(dirty) == 0 {
		fmt.Printf("%s✓ All git repositories are clean.%s\n\n", cGreen, cReset)
	} else {
		fmt.Printf("%s%s✗ %d unclean repositor%s:%s\n\n", cBold, cRed, len(dirty), plural(len(dirty), "y", "ies"), cReset)
		for _, r := range dirty {
			fmt.Printf("  %s%s%s%s  %s(%d change%s)%s\n",
				cBold, cRed, r.name, cReset, cGray, len(r.changes), plural(len(r.changes), "", "s"), cReset)
			for _, ch := range r.changes {
				fmt.Printf("      %s%s%s\n", cYellow, ch, cReset)
			}
			fmt.Println()
		}
	}

	for _, r := range failed {
		fmt.Printf("  %s! %s: %v%s\n", cYellow, r.name, r.err, cReset)
	}

	if verbose {
		for _, r := range clean {
			fmt.Printf("  %s✓ %s (clean)%s\n", cGreen, r.name, cReset)
		}
		for _, r := range skipped {
			fmt.Printf("  %s· %s (not a git repo, skipped)%s\n", cGray, r.name, cReset)
		}
		if len(clean) > 0 || len(skipped) > 0 {
			fmt.Println()
		}
	}

	fmt.Printf("%sSummary:%s %d dirty, %d clean, %d skipped",
		cCyan, cReset, len(dirty), len(clean), len(skipped))
	if len(failed) > 0 {
		fmt.Printf(", %d error(s)", len(failed))
	}
	fmt.Println()
	if !verbose && (len(clean) > 0 || len(skipped) > 0) {
		fmt.Printf("%s(use -v to list clean repos and skipped folders)%s\n", cGray, cReset)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
