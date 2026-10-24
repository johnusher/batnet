package hub

import (
	"os/exec"
	"regexp"
	"testing"
)

// The panel's script runs top to bottom when the page loads: one $("x") that
// finds no element stops it there, and the page stays on "Connecting to the
// radio…" with a working radio behind it (2026-10-24, panel.js asked for
// away-form before index.html had it). Every id the script reaches for by name
// must be on the page, in help.html (loaded into it) or made by the script.
func TestPanelScriptFindsItsElements(t *testing.T) {
	read := func(name string) string {
		b, err := panelFS.ReadFile("panel/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	js := read("panel.js")
	have := map[string]bool{}
	for _, src := range []struct {
		text string
		re   string
	}{
		{read("index.html"), `\bid="([^"]+)"`},
		{read("help.html"), `\bid="([^"]+)"`},
		{js, `\bid:\s*"([^"$]+)"`},
	} {
		for _, m := range regexp.MustCompile(src.re).FindAllStringSubmatch(src.text, -1) {
			have[m[1]] = true
		}
	}

	var want []string
	for _, m := range regexp.MustCompile(`\$\("([^"]+)"\)`).FindAllStringSubmatch(js, -1) {
		want = append(want, m[1])
	}
	// for (const f of ["a", "b"]) $(f)… and setNote("a", …) pass the id along.
	for _, m := range regexp.MustCompile(`for \(const \w+ of \[([^\]]+)\]\)\s*\{?\s*\$\(`).FindAllStringSubmatch(js, -1) {
		for _, s := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
			want = append(want, s[1])
		}
	}
	for _, m := range regexp.MustCompile(`setNote\("([^"]+)"`).FindAllStringSubmatch(js, -1) {
		want = append(want, m[1])
	}
	if len(want) < 100 {
		t.Fatalf("found only %d ids in panel.js: the patterns no longer match the script", len(want))
	}
	for _, id := range want {
		if !have[id] {
			t.Errorf("panel.js uses #%s, which is not on the page", id)
		}
	}
}

// A syntax error stops the whole script the same way, and nothing else here
// runs JavaScript. Checked with node where it is installed.
func TestPageScriptsParse(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	for _, f := range []string{"panel/panel.js", "../web/app/app.js"} {
		if out, err := exec.Command(node, "--check", f).CombinedOutput(); err != nil {
			t.Errorf("%s does not parse:\n%s", f, out)
		}
	}
}
