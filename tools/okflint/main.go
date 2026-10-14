// okflint checks that a directory is a valid Open Knowledge Format (OKF v0.2)
// bundle, plus the batnet conventions on top of it.
//
// Errors (exit 1):
//   - a concept (.md other than index.md/log.md) without frontmatter or `type`
//   - an index.md with frontmatter other than the root's okf_version
//   - a concept or subdirectory not listed in its directory's index.md
//   - a log.md date heading that is not YYYY-MM-DD
//
// Warnings (printed, exit 0):
//   - a bundle-relative link to a concept that does not exist
//   - a concept whose stale_after is in the past
//
// Usage: go run ./tools/okflint knowledge
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	linkRe     = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	logDateRe  = regexp.MustCompile(`^## (.*)$`)
	isoDateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	typeRe     = regexp.MustCompile(`^type:\s*\S`)
	staleRe    = regexp.MustCompile(`^stale_after:\s*(\S+)`)
	reservedMd = map[string]bool{"index.md": true, "log.md": true}
)

type linter struct {
	root     string
	errors   []string
	warnings []string
}

func (l *linter) errf(format string, a ...any)  { l.errors = append(l.errors, fmt.Sprintf(format, a...)) }
func (l *linter) warnf(format string, a ...any) { l.warnings = append(l.warnings, fmt.Sprintf(format, a...)) }

func (l *linter) rel(p string) string {
	r, err := filepath.Rel(l.root, p)
	if err != nil {
		return p
	}
	if r == "." {
		return ""
	}
	return "/" + filepath.ToSlash(r)
}

// frontmatter returns the lines between the opening and closing `---`,
// and whether the file starts with a frontmatter block at all.
func frontmatter(content string) ([]string, bool) {
	sc := bufio.NewScanner(strings.NewReader(content))
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, false
	}
	var lines []string
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "---" {
			return lines, true
		}
		lines = append(lines, sc.Text())
	}
	return nil, false // unterminated
}

func (l *linter) checkConcept(path, content string) {
	fm, ok := frontmatter(content)
	if !ok {
		l.errf("%s: no YAML frontmatter block (OKF §4)", l.rel(path))
		return
	}
	hasType := false
	for _, line := range fm {
		if typeRe.MatchString(line) {
			hasType = true
		}
		if m := staleRe.FindStringSubmatch(line); m != nil {
			if t, err := time.Parse(time.RFC3339, m[1]); err == nil && time.Now().After(t) {
				l.warnf("%s: stale since %s — re-check it, then move stale_after", l.rel(path), m[1])
			}
		}
	}
	if !hasType {
		l.errf("%s: frontmatter has no `type` (the one required OKF field)", l.rel(path))
	}
}

func (l *linter) checkIndex(path, content string, isRoot bool) {
	fm, ok := frontmatter(content)
	if !ok {
		return
	}
	for _, line := range fm {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !(isRoot && strings.HasPrefix(line, "okf_version:")) {
			l.errf("%s: index.md carries no frontmatter, except okf_version at the bundle root (OKF §8)", l.rel(path))
			return
		}
	}
}

func (l *linter) checkLog(path, content string) {
	for _, line := range strings.Split(content, "\n") {
		if m := logDateRe.FindStringSubmatch(line); m != nil && !isoDateRe.MatchString(strings.TrimSpace(m[1])) {
			l.errf("%s: log heading %q is not YYYY-MM-DD (OKF §9)", l.rel(path), m[1])
		}
	}
}

func (l *linter) checkLinks(path, content string) {
	for _, m := range linkRe.FindAllStringSubmatch(content, -1) {
		target := strings.SplitN(m[1], "#", 2)[0]
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		var abs string
		if strings.HasPrefix(target, "/") {
			abs = filepath.Join(l.root, target)
		} else {
			abs = filepath.Join(filepath.Dir(path), target)
		}
		if _, err := os.Stat(abs); err != nil {
			l.warnf("%s: link to %s does not exist yet", l.rel(path), m[1])
		}
	}
}

// checkListed makes sure every concept and subdirectory in dir appears in dir/index.md.
func (l *linter) checkListed(dir string, entries []os.DirEntry) {
	idx, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		l.errf("%s: directory has no index.md", l.rel(dir))
		return
	}
	text := string(idx)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || reservedMd[name] {
			continue
		}
		if e.IsDir() {
			if !strings.Contains(text, "("+name+"/") && !strings.Contains(text, "/"+name+"/") {
				l.errf("%s/index.md: subdirectory %s/ is not listed", l.rel(dir), name)
			}
			continue
		}
		if strings.HasSuffix(name, ".md") && !strings.Contains(text, name) {
			l.errf("%s/index.md: concept %s is not listed", l.rel(dir), name)
		}
	}
}

func (l *linter) walk(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		l.errf("%s: %v", l.rel(dir), err)
		return
	}
	l.checkListed(dir, entries)
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			l.walk(p)
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			l.errf("%s: %v", l.rel(p), err)
			continue
		}
		content := string(b)
		switch e.Name() {
		case "index.md":
			l.checkIndex(p, content, dir == l.root)
		case "log.md":
			l.checkLog(p, content)
		default:
			l.checkConcept(p, content)
		}
		l.checkLinks(p, content)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: okflint <bundle-dir>")
		os.Exit(2)
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	l := &linter{root: root}
	l.walk(root)
	sort.Strings(l.warnings)
	sort.Strings(l.errors)
	for _, w := range l.warnings {
		fmt.Println("warn: ", w)
	}
	for _, e := range l.errors {
		fmt.Println("error:", e)
	}
	if len(l.errors) > 0 {
		fmt.Printf("okflint: %d error(s) in %s\n", len(l.errors), root)
		os.Exit(1)
	}
	fmt.Printf("okflint: %s ok (%d warning(s))\n", root, len(l.warnings))
}
