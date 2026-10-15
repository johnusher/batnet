// Command batnetd is the batnet node daemon. In milestone 1, plan 1 it runs a
// mock village on one machine: houses in a line over an in-process mesh,
// posting messages and printing what arrives where.
package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/mock"
	"github.com/johnusher/batnet/internal/seal"
	"github.com/johnusher/batnet/internal/sim"
)

var houseNames = []string{"Miller", "Fern", "Okafor", "Novak", "Haddad", "Brandt", "Kowalski", "Moreau", "Lindqvist", "Santos"}

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
	flag.Parse()
	if *simFiles {
		os.Exit(runScenarios(flag.Args()))
	}
	if *houses < 2 {
		fmt.Fprintln(os.Stderr, "usage: batnetd -mock N [-loss 0.3] [-for 20s]   (N >= 2)")
		fmt.Fprintln(os.Stderr, "Real radios arrive in plan 4 of milestone 1.")
		os.Exit(2)
	}
	if err := runMock(*houses, *loss, *dur); err != nil {
		log.Fatal(err)
	}
}

func runMock(n int, loss float64, dur time.Duration) error {
	key, err := seal.NewVillageKey()
	if err != nil {
		return err
	}
	m := mock.NewMesh(time.Now().UnixNano())
	m.SetLoss(loss)
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	start := time.Now()
	type node struct {
		name   string
		author *ledger.Author
		sync   *meshsync.Syncer
		store  *ledger.MemStore
	}
	var nodes []*node
	for i := 0; i < n; i++ {
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
				lag := time.Since(time.UnixMilli(e.TS.Wall)).Round(time.Millisecond)
				fmt.Printf("%6s  %-10s got  %s  (after %v)\n", time.Since(start).Round(100*time.Millisecond), name, e.Body, lag)
			},
		})
		if i > 0 {
			m.Connect(nodes[i-1].name, name)
		}
		go nd.sync.Run(ctx)
		nodes = append(nodes, nd)
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
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		sc, err := sim.LoadScenario(b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			return 2
		}
		t0 := time.Now()
		r, _, err := sim.Run(sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			return 2
		}
		fmt.Print(r)
		fmt.Printf("      (%s simulated in %s)\n\n", time.Duration(sc.Duration)*time.Microsecond, time.Since(t0).Round(time.Millisecond))
		if !r.Pass {
			status = 1
		}
	}
	return status
}
