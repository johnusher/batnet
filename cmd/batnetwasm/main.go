//go:build js && wasm

// Command batnetwasm runs batnet's real code for a simulated village inside
// the simulator page. Build: GOOS=js GOARCH=wasm go build -o batnet.wasm ./cmd/batnetwasm
package main

import (
	"syscall/js"

	"github.com/johnusher/batnet/internal/sim"
)

func main() {
	var api sim.API
	errString := func(err error) any {
		if err != nil {
			return err.Error()
		}
		return ""
	}
	js.Global().Set("batnetSim", js.ValueOf(map[string]any{
		"load":  js.FuncOf(func(_ js.Value, args []js.Value) any { return errString(api.Load([]byte(args[0].String()))) }),
		"step":  js.FuncOf(func(_ js.Value, args []js.Value) any { api.Step(int64(args[0].Int())); return nil }),
		"state": js.FuncOf(func(_ js.Value, _ []js.Value) any { return string(api.State()) }),
		"post": js.FuncOf(func(_ js.Value, args []js.Value) any {
			return errString(api.Post(args[0].String(), args[1].String(), args[2].Int()))
		}),
		"power":  js.FuncOf(func(_ js.Value, args []js.Value) any { return errString(api.Power(args[0].String(), args[1].Bool())) }),
		"report": js.FuncOf(func(_ js.Value, _ []js.Value) any { return string(api.Report()) }),
	}))
	select {} // keep the functions alive
}
