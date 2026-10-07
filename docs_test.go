package odbc_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// These tests read the repository and not the package. They keep the
// documents true: a link that breaks, an index that falls behind or a count
// that goes stale is found here and not by a reader.

var (
	markdownLink = regexp.MustCompile(`\]\((?:\./)?([^)#:]+\.md)(#[^)]*)?\)`)
	decisionRef  = regexp.MustCompile(`\bD([1-9][0-9]*)\b`)
	decisionHead = regexp.MustCompile(`\A# D(\d+)\. (.+)\n\nStatus: (.+)\.\n`)
	decisionName = regexp.MustCompile(`^D(\d{3})-[a-z0-9-]+\.md$`)
	// foreignRef is a decision of another project, such as dbimp D135. Each
	// project numbers its own, so the number says nothing about this one.
	foreignRef = regexp.MustCompile(`\b(?:dbimp|dburl|dbmeta|usql)(?:'s)?\s+D[1-9][0-9]*` +
		`(?:(?:,\s*|\s+and\s+|\s+to\s+|\s+or\s+)D[1-9][0-9]*)*`)
)

// decision is one file in docs/decisions.
type decision struct {
	num, title, status, file string
}

// TestEveryMarkdownLinkResolves checks that each document a link names exists.
func TestEveryMarkdownLinkResolves(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md") {
		for _, m := range markdownLink.FindAllStringSubmatch(read(t, path), -1) {
			target := filepath.Join(filepath.Dir(path), m[1])
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: the link to %s does not resolve to %s", path, m[1], target)
			}
		}
	}
}

// decisions reads every file in docs/decisions, in order.
func decisions(t *testing.T) []decision {
	t.Helper()
	dir := filepath.Join("docs", "decisions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []decision
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		name := decisionName.FindStringSubmatch(e.Name())
		if name == nil {
			t.Errorf("%s: a decision file is named D, three digits, a hyphen and the title "+
				"in lower case words, as in D001-the-title.md", e.Name())
			continue
		}
		m := decisionHead.FindStringSubmatch(read(t, filepath.Join(dir, e.Name())))
		if m == nil {
			t.Errorf("%s: a decision opens with \"# D<n>. <title>\", a blank line and "+
				"\"Status: <status>.\"", e.Name())
			continue
		}
		if n, _ := strconv.Atoi(name[1]); strconv.Itoa(n) != m[1] {
			t.Errorf("%s holds D%s. The file name and the heading must name one decision", e.Name(), m[1])
		}
		out = append(out, decision{m[1], m[2], m[3], e.Name()})
	}
	if len(out) == 0 {
		t.Fatal("docs/decisions holds no decision")
	}
	return out
}

// TestEveryDecisionReferenceExists checks that a decision named by number was
// written.
func TestEveryDecisionReferenceExists(t *testing.T) {
	t.Parallel()
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
	}
	for _, path := range repoFiles(t, ".md", ".go", ".yml") {
		body := foreignRef.ReplaceAllString(read(t, path), "")
		for _, m := range decisionRef.FindAllStringSubmatch(body, -1) {
			if !written[m[1]] {
				t.Errorf("%s: names D%s, which is not in docs/decisions", path, m[1])
			}
		}
	}
}

// TestTheDecisionIndexIsComplete checks the table in docs/decisions/README.md
// against the files.
func TestTheDecisionIndexIsComplete(t *testing.T) {
	t.Parallel()
	index := read(t, filepath.Join("docs", "decisions", "README.md"))
	rows := make(map[string]string)
	for _, m := range regexp.MustCompile(`(?m)^\| \[D(\d+)\]\(.*$`).FindAllStringSubmatch(index, -1) {
		rows[m[1]] = m[0]
	}
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
		want := fmt.Sprintf("| [D%s](%s) | %s | %s |", d.num, d.file, d.title, d.status)
		switch got, ok := rows[d.num]; {
		case !ok:
			t.Errorf("D%s has no row in the index. Add:\n%s", d.num, want)
		case got != want:
			t.Errorf("D%s: the row is\n%s\nand the file says\n%s", d.num, got, want)
		}
	}
	for num := range rows {
		if !written[num] {
			t.Errorf("the index has a row for D%s and no file holds it", num)
		}
	}
}

// TestAnAmendmentPointsBothWays checks that a decision that amends another is
// named by the other, so a reader of the older one learns that it changed.
func TestAnAmendmentPointsBothWays(t *testing.T) {
	t.Parallel()
	status := make(map[string]string)
	for _, d := range decisions(t) {
		status[d.num] = d.title + ". " + d.status
	}
	naming := regexp.MustCompile(`(?i)\b(?:amends|supersedes|superseded by|amended by) D(\d+)`)
	for num, head := range status {
		for _, m := range naming.FindAllStringSubmatch(head, -1) {
			other, ok := status[m[1]]
			switch {
			case !ok:
				t.Errorf("D%s names D%s, which is not a decision", num, m[1])
			case !strings.Contains(other, "D"+num):
				t.Errorf("D%s says %q, and D%s does not name D%s", num, head, m[1], num)
			}
		}
	}
}

// TestTheCountsInProseAreRight checks the numbers the documents quote.
func TestTheCountsInProseAreRight(t *testing.T) {
	t.Parallel()
	want := len(decisions(t))
	phrase := regexp.MustCompile(`There are (\d+) decisions so far`)
	var found int
	for _, name := range []string{"README.md", "AGENTS.md"} {
		for _, m := range phrase.FindAllStringSubmatch(read(t, name), -1) {
			found++
			if m[1] != strconv.Itoa(want) {
				t.Errorf("%s says %s decisions and there are %d", name, m[1], want)
			}
		}
	}
	if found == 0 {
		t.Error("no document quotes the number of decisions, so this guards nothing")
	}
}

// TestTheRootHoldsFourDocuments checks that no other document is in the root.
func TestTheRootHoldsFourDocuments(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{
		"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !allowed[e.Name()] {
			t.Errorf("%s is in the root. Only README, AGENTS, CLAUDE and CONTRIBUTING belong there. "+
				"Move it to docs/", e.Name())
		}
	}
	for name := range allowed {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s in the root", name)
		}
	}
}

// TestClaudeImportsAgents checks that CLAUDE.md is a file that holds only
// the import of AGENTS.md. A link does not work, because a Windows checkout
// writes a link as a text file.
func TestClaudeImportsAgents(t *testing.T) {
	t.Parallel()
	info, err := os.Lstat("CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("CLAUDE.md is a symbolic link. Make it a file that holds @AGENTS.md")
	}
	if got := strings.TrimSpace(read(t, "CLAUDE.md")); got != "@AGENTS.md" {
		t.Errorf("CLAUDE.md holds %q. It holds only @AGENTS.md, and the rules go in AGENTS.md", got)
	}
}

// TestEveryDocumentIsInBothTables checks that a document in docs/ is named in
// the table of AGENTS.md and in the one of README.md.
func TestEveryDocumentIsInBothTables(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]string{"AGENTS.md": read(t, "AGENTS.md"), "README.md": read(t, "README.md")}
	var found int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		found++
		for name, body := range tables {
			if !strings.Contains(body, "docs/"+e.Name()) {
				t.Errorf("%s does not name docs/%s", name, e.Name())
			}
		}
	}
	if found == 0 {
		t.Error("docs/ holds no document, so this guards nothing")
	}
}

// TestNoSectionHeadingIsRepeated checks that a level two heading appears once
// in its document.
func TestNoSectionHeadingIsRepeated(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md") {
		seen := make(map[string]bool)
		for _, m := range regexp.MustCompile(`(?m)^## (.+)$`).FindAllStringSubmatch(read(t, path), -1) {
			if seen[m[1]] {
				t.Errorf("%s has two sections called %q", path, m[1])
			}
			seen[m[1]] = true
		}
	}
}

// repoFiles returns every file with one of the extensions. It skips the
// folders that start with a dot, which hold the skills and the git data, and
// keeps .github.
func repoFiles(t *testing.T, exts ...string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if name := d.Name(); name != "." && name != ".github" && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
		default:
			for _, ext := range exts {
				if strings.HasSuffix(path, ext) {
					out = append(out, path)
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
