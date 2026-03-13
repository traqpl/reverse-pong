//go:build js && wasm

package main

import (
	"syscall/js"
)

var engine *Engine

func main() {
	fetchConfig()

	canvas := js.Global().Get("document").Call("getElementById", "gameCanvas")
	engine = NewEngine(canvas)
	engine.registerInput()

	var lastTime float64
	var loop js.Func
	loop = js.FuncOf(func(_ js.Value, args []js.Value) any {
		now := args[0].Float()
		if lastTime == 0 {
			lastTime = now
		}
		dt := (now - lastTime) / 1000.0
		if dt > 0.1 {
			dt = 0.1 // cap to avoid spiral of death on tab switch
		}
		lastTime = now

		engine.Update(dt)
		engine.Render()

		js.Global().Call("requestAnimationFrame", loop)
		return nil
	})

	js.Global().Call("requestAnimationFrame", loop)

	// Block forever so WASM doesn't exit
	select {}
}

// callAudio calls window.audioPlay.<method>() safely.
func callAudio(method string) {
	ap := js.Global().Get("audioPlay")
	if ap.IsUndefined() || ap.IsNull() {
		return
	}
	fn := ap.Get(method)
	if fn.IsUndefined() || fn.IsNull() {
		return
	}
	fn.Call("call", ap)
}
