//go:build js && wasm

// Command batnetwasm runs batnet's real code for a simulated village inside
// the simulator page. Build: GOOS=js GOARCH=wasm go build -o batnet.wasm ./cmd/batnetwasm
package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/johnusher/batnet/internal/sim"
)

// fn wraps a bridge function: it checks the argument count and turns a panic
// into an error string, so one bad call from the page can't stop the program.
func fn(nargs int, f func(args []js.Value) any) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) (out any) {
		defer func() {
			if r := recover(); r != nil {
				out = fmt.Sprintf("batnet: %v", r)
			}
		}()
		if len(args) < nargs {
			return fmt.Sprintf("batnet: %d arguments needed", nargs)
		}
		return f(args)
	})
}

func errString(err error) any {
	if err != nil {
		return err.Error()
	}
	return ""
}

func jsonString(v any) any {
	b, _ := json.Marshal(v)
	return string(b)
}

func main() {
	var api sim.API
	js.Global().Set("batnetSim", js.ValueOf(map[string]any{
		"load":   fn(1, func(a []js.Value) any { return errString(api.Load([]byte(a[0].String()))) }),
		"step":   fn(1, func(a []js.Value) any { api.Step(int64(a[0].Float())); return nil }),
		"state":  fn(0, func(a []js.Value) any { return string(api.State()) }),
		"post":   fn(3, func(a []js.Value) any { return errString(api.Post(a[0].String(), a[1].String(), a[2].Int())) }),
		"power":  fn(2, func(a []js.Value) any { return errString(api.Power(a[0].String(), a[1].Bool())) }),
		"report": fn(0, func(a []js.Value) any { return string(api.Report()) }),
		"people": fn(0, func(a []js.Value) any { return string(api.People()) }),
		"login": fn(2, func(a []js.Value) any {
			tok, err := api.Login(a[0].String(), a[1].String())
			if err != nil {
				return jsonString(map[string]string{"error": err.Error()})
			}
			return jsonString(map[string]string{"token": tok})
		}),
		"http": fn(5, func(a []js.Value) any {
			status, tok, body := api.HTTP(a[0].String(), a[1].String(), a[2].String(), a[3].String(), a[4].String())
			return jsonString(map[string]any{"status": status, "token": tok, "body": body})
		}),
		"events": fn(2, func(a []js.Value) any { return string(api.Events(a[0].String(), a[1].String())) }),
	}))
	select {} // keep the functions alive
}
