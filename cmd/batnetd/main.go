// Command batnetd is the batnet node daemon. -hub runs a house hub on a Pi
// (the app, the control panel, the radio, the WiFi mesh); -mock runs a mock
// village on one machine; -sim judges village scenarios.
package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/mock"
	"github.com/johnusher/batnet/internal/seal"
	"github.com/johnusher/batnet/internal/sim"
	"github.com/johnusher/batnet/internal/state"
	"github.com/johnusher/batnet/internal/web"
)

var houseNames = []string{"Usher", "Shemie", "Dittrich", "Green", "Willett", "Seigner", "Chasserot", "Bates", "Crane", "Ferguson", "Elster", "Thornhill", "Kendall", "Jefferies", "Fremont", "Danvers", "Daniels", "Goldfinger", "Walpole", "Blofeld", "Pitt", "Klebb", "Peel", "Largo", "Disraeli", "Scaramanga", "Gladstone", "Stromberg", "Salisbury", "Drax", "Balfour", "Khan", "Asquith", "Zorin", "Baldwin", "Sanchez", "MacDonald", "Trevelyan", "Chamberlain", "Onatopp", "Churchill", "Carver", "Attlee", "King", "Eden", "Graves", "Macmillan", "Silva", "Wilson", "Safin", "Heath", "Grant", "Callaghan", "Thatcher", "Major", "Blair", "Brown", "May", "Starmer", "Sunak"}

type post struct {
	Audience string `json:"audience"`
	From     string `json:"from"`
	Text     string `json:"text"`
}

func main() {
	houses := flag.Int("mock", 0, "run a mock village with this many houses in a line")
	loss := flag.Float64("loss", 0, "chance a broadcast misses each house (0-1)")
	dur := flag.Duration("for", 15*time.Second, "how long the mock village runs")
	simFiles := flag.Bool("sim", false, "run the scenario files given as arguments and judge them")
	withWeb := flag.Bool("web", false, "with -mock: serve the batnet app for every house on 127.0.0.1:8001 and up")
	asHub := flag.Bool("hub", false, "run as a house hub on a Pi: the house app, the control panel on /hub/, the radio and the WiFi mesh")
	data := flag.String("data", "/var/lib/batnet", "with -hub: the folder for the log, keys, PINs and settings")
	listen := flag.String("listen", ":80", "with -hub: the address the app and panel listen on")
	radio := flag.String("radio", "127.0.0.1:4403", "with -hub: meshtasticd's TCP API")
	meshMode := flag.String("mode", "", "with -hub: start the WiFi mesh in this mode (off, mock or real) and keep it; default: the saved mode")
	house := flag.String("house", "", "with -hub: the house's name (saved)")
	flag.Parse()
	if *simFiles {
		os.Exit(runScenarios(flag.Args()))
	}
	if *asHub {
		err := runHub(*data, *listen, *radio, *meshMode, *house)
		if errors.Is(err, errRestart) {
			restartSelf()
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	if *houses < 2 {
		fmt.Fprintln(os.Stderr, "usage: batnetd -mock N [-loss 0.3] [-for 20s] [-web]   (N >= 2; -for 0 runs until Ctrl-C)")
		fmt.Fprintln(os.Stderr, "       batnetd -sim scenarios/*.json")
		fmt.Fprintln(os.Stderr, "       batnetd -hub [-data /var/lib/batnet] [-listen :80] [-mode off|mock|real]")
		os.Exit(2)
	}
	if err := runMock(*houses, *loss, *dur, *withWeb); err != nil {
		log.Fatal(err)
	}
}

// node is one mock house.
type node struct {
	name   string
	author *ledger.Author
	sync   *meshsync.Syncer
	store  *ledger.MemStore
	web    *web.Server
}

// serveMockWeb bootstraps every house and serves its app on 127.0.0.1:8001 and up until ctx ends.
func serveMockWeb(ctx context.Context, nodes []*node) error {
	var servers []*http.Server
	for i, nd := range nodes {
		groups := []string(nil)
		if i == 0 {
			groups = []string{"Medical", "Food"} // groups are village-wide: create them once
		}
		if _, err := nd.web.Bootstrap("Admin", "123456", groups); err != nil { // placeholder PIN, mock only
			return err
		}
		addr := fmt.Sprintf("127.0.0.1:%d", 8001+i)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		hs := &http.Server{Handler: nd.web.Handler(), ReadHeaderTimeout: 10 * time.Second}
		servers = append(servers, hs)
		go hs.Serve(ln)
		fmt.Printf("  %-10s http://%s   (house admin: Admin / PIN 123456)\n", nd.name, addr)
	}
	fmt.Println("Each address is one house. Sign up at one, approve as Admin, post, and watch it arrive at the others. Ctrl-C stops.")
	<-ctx.Done()
	for _, hs := range servers {
		hs.Close()
	}
	return nil
}

func runMock(n int, loss float64, dur time.Duration, withWeb bool) error {
	key, err := seal.NewVillageKey()
	if err != nil {
		return err
	}
	m := mock.NewMesh(time.Now().UnixNano())
	m.SetLoss(loss)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if dur > 0 {
		ctx, cancel = context.WithTimeout(ctx, dur)
		defer cancel()
	}
	start := time.Now()
	var nodes []*node
	for i := range n {
		name := houseNames[i%len(houseNames)]
		if i >= len(houseNames) {
			name = fmt.Sprintf("%s-%d", name, i/len(houseNames)+1)
		}
		nd := &node{name: name, store: ledger.NewMemStore()}
		_, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		clock := hlc.New(func() int64 { return time.Now().UnixMilli() })
		if nd.author, err = ledger.NewAuthor(priv, clock, nd.store); err != nil {
			return err
		}
		sealer, err := seal.NewSealer(key, seal.PurposeMesh)
		if err != nil {
			return err
		}
		link := m.Add(name)
		nd.sync = meshsync.New(meshsync.Config{
			Link: link, Store: nd.store, Clock: clock, Sealer: sealer,
			Heartbeat: 500 * time.Millisecond,
			OnEntry: func(e *ledger.Entry) {
				if nd.web != nil {
					nd.web.OnEntry(e)
					return
				}
				lag := time.Since(time.UnixMilli(e.TS.Wall)).Round(time.Millisecond)
				fmt.Printf("%6s  %-10s got  %s  (after %v)\n", time.Since(start).Round(100*time.Millisecond), name, e.Body, lag)
			},
		})
		if withWeb {
			nd.web = web.New(web.Config{House: name, Author: nd.author, State: state.New(),
				Publish: func(e *ledger.Entry) { nd.sync.Publish(e) }})
		}
		if i > 0 {
			m.Connect(nodes[i-1].name, name)
		}
		go nd.sync.Run(ctx)
		nodes = append(nodes, nd)
	}
	if withWeb {
		return serveMockWeb(ctx, nodes)
	}
	fmt.Printf("Mock village: %d houses in a line, %.0f%% broadcast loss, %v.\n", n, loss*100, dur)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("\nWhat each house holds:")
			for _, nd := range nodes {
				all, _ := nd.store.All()
				fmt.Printf("  %-10s %d entries\n", nd.name, len(all))
			}
			return nil
		case <-tick.C:
			nd := nodes[rand.Intn(len(nodes))]
			e, err := nd.author.Append(ledger.TypeMsgPost, 5, post{Audience: "everyone", From: nd.name, Text: fmt.Sprintf("message %d", i)})
			if err != nil {
				return err
			}
			fmt.Printf("%6s  %-10s sent %s\n", time.Since(start).Round(100*time.Millisecond), nd.name, e.Body)
			nd.sync.Publish(e)
		}
	}
}

// runScenarios runs each scenario file and prints its report; 1 if any fails.
func runScenarios(paths []string) int {
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: batnetd -sim scenarios/bang.json [more.json ...]")
		return 2
	}
	status := 0
	for _, p := range paths {
		if filepath.Base(p) == "index.json" {
			continue // the simulator page's list of scenarios, not a scenario
		}
		// One bad file is reported and the rest still run (B4); the exit status says it was bad.
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			status = 2
			continue
		}
		sc, err := sim.LoadScenario(b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			status = 2
			continue
		}
		t0 := time.Now()
		r, _, err := sim.Run(sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			status = 2
			continue
		}
		fmt.Print(r)
		fmt.Printf("      (%s simulated in %s)\n\n", time.Duration(sc.Duration)*time.Microsecond, time.Since(t0).Round(time.Millisecond))
		if !r.OK() && status == 0 {
			status = 1
		}
	}
	return status
}

// restartSelf starts batnetd again in place, in the mode just saved: the
// same command without -mode, which would override the saved choice.
func restartSelf() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	var args []string
	for i := 0; i < len(os.Args); i++ {
		a := os.Args[i]
		if a == "-mode" || a == "--mode" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-mode=") || strings.HasPrefix(a, "--mode=") {
			continue
		}
		args = append(args, a)
	}
	log.Printf("restarting in the new mode")
	log.Fatal(syscall.Exec(exe, args, os.Environ()))
}
