package translate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMain doubles as a pretend helper: with BATNET_FAKE_TRANSLATE set it
// answers by upper-casing the text.
func TestMain(m *testing.M) {
	if os.Getenv("BATNET_FAKE_TRANSLATE") == "1" {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			var r request
			json.Unmarshal(sc.Bytes(), &r)
			if r.Text == "boom" {
				os.Exit(3)
			}
			b, _ := json.Marshal(answer{ID: r.ID, Text: fmt.Sprintf("[%s>%s] %s", r.From, r.To, strings.ToUpper(r.Text))})
			fmt.Println(string(b))
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeClient(t *testing.T) *Client {
	t.Helper()
	models := t.TempDir()
	os.Mkdir(models+"/de-en", 0o755)
	os.Mkdir(models+"/en-de", 0o755)
	os.Mkdir(models+"/notapair", 0o755)
	t.Setenv("BATNET_FAKE_TRANSLATE", "1")
	c := New(os.Args[0], models)
	c.Args = []string{"-test.run=^$"}
	return c
}

func TestTranslateThroughHelper(t *testing.T) {
	c := fakeClient(t)
	if got := strings.Join(c.Pairs(), ","); got != "de-en,en-de" {
		t.Fatal(got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got, err := c.Translate(ctx, "de", "en", "Wasser ist da")
	if err != nil || got != "[de>en] WASSER IST DA" {
		t.Fatalf("%q %v", got, err)
	}
	if !c.Status().Loaded {
		t.Fatal("the helper should stay loaded")
	}
	if _, err := c.Translate(ctx, "uk", "en", "привіт"); err == nil || !strings.Contains(err.Error(), "Ukrainian") {
		t.Fatalf("missing pair: %v", err)
	}
	// The helper dies: an error, then a fresh start.
	if _, err := c.Translate(ctx, "de", "en", "boom"); err == nil {
		t.Fatal("no error when the helper died")
	}
	got, err = c.Translate(ctx, "en", "de", "hello")
	if err != nil || got != "[en>de] HELLO" {
		t.Fatalf("after a restart: %q %v", got, err)
	}
	c.mu.Lock()
	c.stop()
	c.mu.Unlock()
	// Cached: no helper needed.
	if got, _ := c.Translate(ctx, "de", "en", "Wasser ist da"); got != "[de>en] WASSER IST DA" || c.Status().Loaded {
		t.Fatalf("cache: %q", got)
	}
}

func TestGuess(t *testing.T) {
	for text, want := range map[string]string{
		"Wasser ist bei uns wieder da.":            "de",
		"Hat jemand eine Ersatzbatterie für 12 V?": "de",
		"Grüße aus Kreuzberg":                      "de",
		"Water is back on at our end.":             "en",
		"Anyone have a spare 12 V battery?":        "en",
		"👍":                                        "",
		"73":                                       "",
	} {
		if got := Guess(text); got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
}
