// Package translate runs the hub's offline translator (decision 0030): a
// helper process (deploy/pi/batnet-translate, Argos models on CTranslate2)
// spoken to in JSON lines, started on first use and stopped when idle; and a
// word-list guess at a message's language.
package translate

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Defaults on the Pi (deploy/pi/translate-setup.sh).
const (
	DefaultHelper = "/usr/local/lib/batnet/batnet-translate"
	DefaultModels = "/var/lib/batnet-translate/models"
)

// Status is what the panel shows.
type Status struct {
	Installed bool     `json:"installed"`
	Pairs     []string `json:"pairs"` // like de-en
	Loaded    bool     `json:"loaded"`
	Error     string   `json:"error,omitempty"`
}

// Client talks to the helper.
type Client struct {
	Helper string   // the helper's path
	Args   []string // before --models
	Models string   // the models folder
	// IdleStop is how long the helper may sit unused before it is stopped
	// (nil: 10 minutes).
	IdleStop func() time.Duration

	mu      sync.Mutex
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     *bufio.Scanner
	id      int
	lastUse time.Time
	err     string
	cache   map[string]string
	order   []string
}

// New returns a client for the helper and models folder (empty: the defaults).
func New(helper, models string) *Client {
	if helper == "" {
		helper = DefaultHelper
	}
	if models == "" {
		models = DefaultModels
	}
	return &Client{Helper: helper, Models: models}
}

var pairRe = regexp.MustCompile(`^[a-z]{2,3}-[a-z]{2,3}$`)

// Pairs lists the installed language pairs, from the models folder.
func (c *Client) Pairs() []string {
	ds, _ := os.ReadDir(c.Models)
	var out []string
	for _, d := range ds {
		if d.IsDir() && pairRe.MatchString(d.Name()) {
			out = append(out, d.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Has reports whether from→to is installed.
func (c *Client) Has(from, to string) bool {
	for _, p := range c.Pairs() {
		if p == from+"-"+to {
			return true
		}
	}
	return false
}

// Status returns what the panel shows.
func (c *Client) Status() Status {
	_, err := os.Stat(c.Helper)
	st := Status{Installed: err == nil, Pairs: c.Pairs()}
	if st.Pairs == nil {
		st.Pairs = []string{}
	}
	c.mu.Lock()
	st.Loaded, st.Error = c.cmd != nil, c.err
	c.mu.Unlock()
	return st
}

type request struct {
	ID   int    `json:"id"`
	Op   string `json:"op,omitempty"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	Text string `json:"text,omitempty"`
}

type answer struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Error string `json:"error"`
}

// Translate translates text from one language to another ("de", "en").
func (c *Client) Translate(ctx context.Context, from, to, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("Nothing to translate.")
	}
	if len(text) > 2000 {
		return "", errors.New("That is too long to translate here.")
	}
	if from == to {
		return text, nil
	}
	if !c.Has(from, to) {
		return "", fmt.Errorf("No %s to %s translator is installed on this hub.", Name(from), Name(to))
	}
	key := from + "|" + to + "|" + text
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.cache[key]; ok {
		return t, nil
	}
	if err := c.start(); err != nil {
		c.err = err.Error()
		return "", errors.New("The translator did not start: " + err.Error())
	}
	c.id++
	req := request{ID: c.id, From: from, To: to, Text: text}
	b, _ := json.Marshal(req)
	type res struct {
		a   answer
		err error
	}
	done := make(chan res, 1)
	go func(out *bufio.Scanner) {
		for out.Scan() {
			var a answer
			if json.Unmarshal(out.Bytes(), &a) == nil && a.ID == req.ID {
				done <- res{a: a}
				return
			}
		}
		done <- res{err: errors.New("the translator stopped")}
	}(c.out)
	if _, err := c.in.Write(append(b, '\n')); err != nil {
		c.stop()
		return "", errors.New("The translator stopped: " + err.Error())
	}
	var r res
	select {
	case r = <-done:
	case <-ctx.Done():
		c.stop() // the reader goroutine ends with the process
		return "", errors.New("The translation took too long.")
	}
	c.lastUse = time.Now()
	if r.err != nil {
		c.stop()
		c.err = r.err.Error()
		return "", errors.New("The translator stopped.")
	}
	if r.a.Error != "" {
		return "", errors.New(r.a.Error)
	}
	c.err = ""
	c.remember(key, r.a.Text)
	return r.a.Text, nil
}

func (c *Client) remember(key, v string) { // caller holds mu
	if c.cache == nil {
		c.cache = map[string]string{}
	}
	if _, ok := c.cache[key]; !ok {
		c.order = append(c.order, key)
	}
	c.cache[key] = v
	for len(c.order) > 300 {
		delete(c.cache, c.order[0])
		c.order = c.order[1:]
	}
}

func (c *Client) start() error { // caller holds mu
	if c.cmd != nil {
		return nil
	}
	cmd := exec.Command(c.Helper, append(append([]string{}, c.Args...), "--models", c.Models)...)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	c.cmd, c.in, c.out, c.lastUse = cmd, in, sc, time.Now()
	return nil
}

func (c *Client) stop() { // caller holds mu
	if c.cmd == nil {
		return
	}
	c.in.Close()
	c.cmd.Process.Kill()
	c.cmd.Wait()
	c.cmd, c.in, c.out = nil, nil, nil
}

// Run stops the helper when it has been idle, until ctx ends.
func (c *Client) Run(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.stop()
		c.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
		idle := 10 * time.Minute
		if c.IdleStop != nil {
			idle = c.IdleStop()
		}
		// TryLock: a translation under way is not idle.
		if c.mu.TryLock() {
			if c.cmd != nil && time.Since(c.lastUse) > idle {
				c.stop()
			}
			c.mu.Unlock()
		}
	}
}

// ---- the language guess ----

var words = map[string]map[string]bool{
	"de": set("der die das und ist nicht ich du wir ihr sie es ein eine einen mit von zu auf für bei hat haben habe " +
		"wer was wo wie warum noch schon auch aber oder wenn dann bitte danke hallo gibt jemand kein keine strom " +
		"wasser heute morgen gestern hier dort sind bin bist war mein meine dein uns euch im am zum zur über nach " +
		"gut ja nein mal gerade wieder alles alle viel kann können muss soll gute grüße moin servus tschüss"),
	"en": set("the and is not you we they it a an with from to on for at has have had who what where how why " +
		"still already also but or if then please thanks hello anyone any no power water today tomorrow yesterday " +
		"here there are am was my your our us in of by after good yes can could must should all much just again " +
		"this that hi"),
}

func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

var wordRe = regexp.MustCompile(`[\p{L}']+`)

// Guess guesses the language of a short message from its common words:
// "de", "en" or "" when it can't tell.
func Guess(text string) string {
	score := map[string]int{}
	for _, w := range wordRe.FindAllString(strings.ToLower(text), -1) {
		for lang, ws := range words {
			if ws[w] {
				score[lang]++
			}
		}
		if strings.ContainsAny(w, "äöüß") {
			score["de"]++
		}
	}
	switch {
	case score["de"] > score["en"]:
		return "de"
	case score["en"] > score["de"]:
		return "en"
	}
	return ""
}

var names = map[string]string{"de": "German", "en": "English", "uk": "Ukrainian", "ru": "Russian", "pl": "Polish",
	"tr": "Turkish", "ar": "Arabic", "fr": "French", "es": "Spanish", "it": "Italian"}

// Name is a language's English name.
func Name(code string) string {
	if n, ok := names[code]; ok {
		return n
	}
	return code
}

// ModelsPath joins the models folder and a pair (for tests and the setup).
func (c *Client) ModelsPath(pair string) string { return filepath.Join(c.Models, pair) }
