package hub

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SoundSettings are the hub's sounds (decision 0018's house speaker). No
// omitempty: a false must survive a restart, and missing keys keep the defaults.
type SoundSettings struct {
	On        bool   `json:"on"`
	Output    string `json:"output"`    // auto (a USB speaker if there is one, else the jack), jack, usb
	Volume    int    `json:"volume"`    // 10 to 100 %
	Direct    bool   `json:"direct"`    // a message for this hub
	Public    bool   `json:"public"`    // a message to everyone (at most one chime in 30 s)
	Replies   bool   `json:"replies"`   // an answer to one of the hub's messages
	Delivered bool   `json:"delivered"` // a message of the hub's arrived (or failed)
	Speak     bool   `json:"speak"`     // say who it is from
	ReadAloud bool   `json:"readAloud"` // and read it out
	Quiet     bool   `json:"quiet"`     // quiet hours: only messages for the hub sound
	QuietFrom string `json:"quietFrom"`
	QuietTo   string `json:"quietTo"`
}

// DefaultSound is the hub's sound out of the box: on, the jack (or a USB
// speaker), a chime for messages and answers, no speech.
var DefaultSound = SoundSettings{On: true, Output: "auto", Volume: 80, Direct: true, Public: true, Replies: true,
	QuietFrom: "22:00", QuietTo: "07:00"}

// SoundStatus is what the panel shows.
type SoundStatus struct {
	Settings SoundSettings `json:"settings"`
	Output   string        `json:"output"` // jack, usb or "" (none found)
	Card     string        `json:"card,omitempty"`
	Cards    []SoundCard   `json:"cards"`
	CanSpeak bool          `json:"canSpeak"` // espeak-ng is installed
	Last     time.Time     `json:"last,omitempty"`
	LastWhat string        `json:"lastWhat,omitempty"`
	Error    string        `json:"error,omitempty"`
	Quiet    bool          `json:"quietNow"`
}

// SoundCard is one ALSA card.
type SoundCard struct {
	Index  int    `json:"index"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Kind   string `json:"kind"` // jack, usb, hdmi
}

// Sound plays the hub's chimes and speech.
type Sound struct {
	run   Runner
	play  func(ctx context.Context, device string, wav []byte) error
	speak func(ctx context.Context, text string) ([]byte, error)
	cards func() []SoundCard
	save  func(SoundSettings) error
	now   func() time.Time

	q chan soundJob

	mu         sync.Mutex
	set        SoundSettings
	last       time.Time
	lastWhat   string
	lastPublic time.Time
	err        string
}

type soundJob struct {
	kind, say string
}

// SoundConfig wires a Sound.
type SoundConfig struct {
	Run      Runner
	Settings SoundSettings
	Save     func(SoundSettings) error
}

// NewSound returns the hub's sound; Run plays.
func NewSound(c SoundConfig) *Sound {
	s := &Sound{run: c.Run, set: c.Settings, save: c.Save, now: time.Now, q: make(chan soundJob, 4),
		play: aplay, speak: espeak, cards: readSoundCards}
	return s
}

// Run plays what is queued until ctx ends.
func (s *Sound) Run(ctx context.Context) {
	s.applyVolume()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-s.q:
			s.playJob(ctx, j)
		}
	}
}

// Notify sounds for an event: direct, public, reply, delivered, failed,
// wifi-on, wifi-off. who and text are for speech.
func (s *Sound) Notify(kind, who, text string) {
	s.mu.Lock()
	set := s.set
	now := s.now()
	ok := set.On
	switch kind {
	case "direct":
		ok = ok && set.Direct
	case "public":
		ok = ok && set.Public && now.Sub(s.lastPublic) >= 30*time.Second
		if ok {
			s.lastPublic = now
		}
	case "reply":
		ok = ok && set.Replies
	case "delivered", "failed":
		ok = ok && set.Delivered
	}
	if ok && kind != "direct" && !strings.HasPrefix(kind, "wifi") && quietNow(set, now) {
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return
	}
	say := ""
	if set.Speak && (kind == "direct" || kind == "public" || kind == "reply") {
		switch kind {
		case "direct":
			say = "Message from " + speakable(who)
		case "reply":
			say = speakable(who) + " answered"
		default:
			say = speakable(who) + " to everyone"
		}
		if set.ReadAloud && text != "" {
			say += ". " + text
		}
	}
	if strings.HasPrefix(kind, "wifi") && set.Speak {
		say = map[string]string{"wifi-on": "The hub's own WiFi is on", "wifi-off": "Back on the home WiFi"}[kind]
	}
	s.enqueue(soundJob{kind: kind, say: say})
}

// Test plays the test chime and a sentence, whatever the settings.
func (s *Sound) Test() {
	s.enqueue(soundJob{kind: "test", say: "This is the hub's speaker."})
}

func (s *Sound) enqueue(j soundJob) {
	select {
	case s.q <- j:
	default: // a backlog of chimes helps nobody
	}
}

func (s *Sound) playJob(ctx context.Context, j soundJob) {
	dev, card := s.device()
	s.mu.Lock()
	s.last, s.lastWhat = s.now(), j.kind
	s.mu.Unlock()
	if dev == "" {
		s.setErr("No sound output found: the Pi 4's audio jack or a USB speaker.")
		return
	}
	c, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := s.play(c, dev, Chime(j.kind))
	if err == nil && j.say != "" && s.speak != nil {
		var wav []byte
		if wav, err = s.speak(c, j.say); err == nil {
			err = s.play(c, dev, wav)
		} else if errors.Is(err, exec.ErrNotFound) {
			err = nil // no espeak-ng: the chime is enough
		}
	}
	if err != nil {
		s.setErr("Playing on " + card + ": " + err.Error())
		return
	}
	s.setErr("")
}

// device picks the ALSA device for the setting.
func (s *Sound) device() (dev, card string) {
	s.mu.Lock()
	out := s.set.Output
	s.mu.Unlock()
	c := pickCard(s.cards(), out)
	if c == nil {
		return "", ""
	}
	return "plughw:CARD=" + c.ID + ",DEV=0", c.Name
}

func pickCard(cards []SoundCard, out string) *SoundCard {
	var jack, usb *SoundCard
	for i := range cards {
		switch cards[i].Kind {
		case "jack":
			jack = &cards[i]
		case "usb":
			if usb == nil {
				usb = &cards[i]
			}
		}
	}
	switch out {
	case "jack":
		return jack
	case "usb":
		return usb
	}
	if usb != nil {
		return usb
	}
	return jack
}

// applyVolume sets the mixer of the chosen card and unmutes it.
func (s *Sound) applyVolume() {
	s.mu.Lock()
	vol, out := s.set.Volume, s.set.Output
	s.mu.Unlock()
	c := pickCard(s.cards(), out)
	if c == nil || s.run == nil {
		return
	}
	if vol < 10 || vol > 100 {
		vol = 80
	}
	for _, ctl := range []string{"PCM", "Speaker", "Master", "Headphone"} {
		if _, err := runFor(s.run, 5*time.Second, "amixer", "-q", "-c", c.ID, "sset", ctl, strconv.Itoa(vol)+"%", "unmute"); err == nil {
			return
		}
	}
}

// Configure changes the sound settings.
func (s *Sound) Configure(n SoundSettings) error {
	if n.Output != "auto" && n.Output != "jack" && n.Output != "usb" {
		return errors.New("Sound goes to auto, the jack or a USB speaker.")
	}
	if n.Volume < 10 || n.Volume > 100 {
		return errors.New("Volume is 10 to 100 %.")
	}
	if _, ok := hhmm(n.QuietFrom); !ok {
		return errors.New("Quiet from: a time like 22:00.")
	}
	if _, ok := hhmm(n.QuietTo); !ok {
		return errors.New("Quiet until: a time like 07:00.")
	}
	s.mu.Lock()
	s.set = n
	s.mu.Unlock()
	s.applyVolume()
	if s.save != nil {
		return s.save(n)
	}
	return nil
}

// Settings returns the sound settings.
func (s *Sound) Settings() SoundSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set
}

// Status returns what the panel shows.
func (s *Sound) Status() SoundStatus {
	cards := s.cards()
	s.mu.Lock()
	defer s.mu.Unlock()
	st := SoundStatus{Settings: s.set, Cards: cards, Last: s.last, LastWhat: s.lastWhat, Error: s.err, Quiet: quietNow(s.set, s.now())}
	if c := pickCard(cards, s.set.Output); c != nil {
		st.Output, st.Card = c.Kind, c.Name
	}
	_, err := exec.LookPath("espeak-ng")
	st.CanSpeak = err == nil
	return st
}

func (s *Sound) setErr(e string) {
	s.mu.Lock()
	s.err = e
	s.mu.Unlock()
}

func hhmm(v string) (int, bool) {
	h, m, ok := strings.Cut(v, ":")
	hi, e1 := strconv.Atoi(h)
	mi, e2 := strconv.Atoi(m)
	if !ok || e1 != nil || e2 != nil || hi < 0 || hi > 23 || mi < 0 || mi > 59 {
		return 0, false
	}
	return hi*60 + mi, true
}

// quietNow reports whether now is inside the quiet hours (which may wrap midnight).
func quietNow(s SoundSettings, now time.Time) bool {
	if !s.Quiet {
		return false
	}
	from, ok1 := hhmm(s.QuietFrom)
	to, ok2 := hhmm(s.QuietTo)
	if !ok1 || !ok2 || from == to {
		return false
	}
	m := now.Hour()*60 + now.Minute()
	if from < to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

// speakable keeps a name's letters and digits, so espeak doesn't spell out emoji.
func speakable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x2000 {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" || strings.HasPrefix(out, "!") {
		return "a radio"
	}
	return out
}

// readSoundCards lists /proc/asound/cards.
func readSoundCards() []SoundCard {
	f, err := os.Open("/proc/asound/cards")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []SoundCard
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		c, ok := parseCardLine(sc.Text())
		if ok {
			out = append(out, c)
		}
	}
	return out
}

// parseCardLine reads " 0 [Headphones     ]: bcm2835_headpho - bcm2835 Headphones".
func parseCardLine(l string) (SoundCard, bool) {
	l = strings.TrimSpace(l)
	i := strings.IndexByte(l, '[')
	j := strings.Index(l, "]:")
	if i < 1 || j < i {
		return SoundCard{}, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(l[:i]))
	if err != nil {
		return SoundCard{}, false
	}
	c := SoundCard{Index: n, ID: strings.TrimSpace(l[i+1 : j])}
	rest := strings.TrimSpace(l[j+2:])
	c.Driver, c.Name, _ = strings.Cut(rest, " - ")
	c.Driver, c.Name = strings.TrimSpace(c.Driver), strings.TrimSpace(c.Name)
	switch {
	case c.ID == "Headphones" || strings.HasPrefix(c.Driver, "bcm2835"):
		c.Kind = "jack"
	case c.Driver == "USB-Audio":
		c.Kind = "usb"
	case strings.HasPrefix(c.ID, "vc4hdmi"):
		c.Kind = "hdmi"
	}
	return c, true
}

func aplay(ctx context.Context, device string, wav []byte) error {
	cmd := exec.CommandContext(ctx, "aplay", "-q", "-D", device, "-")
	cmd.Stdin = bytes.NewReader(wav)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) > 0 {
		return errors.New(firstLine(strings.TrimSpace(string(out))))
	}
	return err
}

func espeak(ctx context.Context, text string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "espeak-ng", "--stdout", "-v", "en", "-s", "150", text)
	return cmd.Output()
}

// ---- the chimes, made here so nothing has to be installed ----

const sampleRate = 22050

type note struct {
	at, freq, dur, gain float64 // seconds, Hz, seconds, 0..1
}

var chimes = map[string][]note{
	"direct":    {{0, 659.3, 0.5, 0.8}, {0.16, 784.0, 0.5, 0.8}, {0.32, 1046.5, 0.8, 0.8}},
	"public":    {{0, 784.0, 0.45, 0.45}, {0.18, 659.3, 0.6, 0.45}},
	"reply":     {{0, 1046.5, 0.3, 0.6}, {0.09, 1318.5, 0.5, 0.6}},
	"delivered": {{0, 1760.0, 0.15, 0.35}},
	"failed":    {{0, 440.0, 0.4, 0.5}, {0.2, 349.2, 0.6, 0.5}},
	"wifi-on":   {{0, 523.3, 0.3, 0.6}, {0.12, 659.3, 0.3, 0.6}, {0.24, 784.0, 0.5, 0.6}},
	"wifi-off":  {{0, 784.0, 0.3, 0.6}, {0.12, 659.3, 0.3, 0.6}, {0.24, 523.3, 0.5, 0.6}},
	"test":      {{0, 659.3, 0.5, 0.8}, {0.16, 784.0, 0.5, 0.8}, {0.32, 1046.5, 0.8, 0.8}},
}

// Chime returns a kind's chime as a WAV file (16-bit mono).
func Chime(kind string) []byte {
	notes, ok := chimes[kind]
	if !ok {
		notes = chimes["direct"]
	}
	end := 0.0
	for _, n := range notes {
		end = math.Max(end, n.at+n.dur)
	}
	samples := make([]float64, int((end+0.05)*sampleRate))
	for _, n := range notes {
		start := int(n.at * sampleRate)
		for i := 0; i < int(n.dur*sampleRate) && start+i < len(samples); i++ {
			t := float64(i) / sampleRate
			// A bell: a quick attack, an exponential fade, a little of the octave.
			env := math.Min(1, t/0.005) * math.Exp(-t*6/n.dur)
			v := math.Sin(2*math.Pi*n.freq*t) + 0.25*math.Sin(4*math.Pi*n.freq*t)
			samples[start+i] += n.gain * env * v / 1.25
		}
	}
	return wav(samples)
}

func wav(samples []float64) []byte {
	data := len(samples) * 2
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+data))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(sampleRate), uint32(sampleRate * 2), uint16(2), uint16(16)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(data))
	for _, v := range samples {
		v = math.Max(-1, math.Min(1, v))
		binary.Write(&b, binary.LittleEndian, int16(v*32000))
	}
	return b.Bytes()
}
