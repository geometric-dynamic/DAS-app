package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestWailsStateEvents(t *testing.T) {
	a := NewApp()
	a.wails = application.New(application.Options{Name: "DAS test"})
	events := make(chan *application.CustomEvent, 4)
	for _, name := range []string{"new-data", "discovered-nodes", "app-state", "recording-state"} {
		cancel := a.wails.Event.On(name, func(event *application.CustomEvent) { events <- event })
		t.Cleanup(cancel)
	}
	state := a.GetFrontendState()
	want := map[string]any{
		"new-data":         state.Nodes,
		"discovered-nodes": state.DiscoveredNodes,
		"app-state":        state.AppState,
		"recording-state":  false,
	}
	a.NewDataNotify()
	a.StopRecording()
	for range 4 {
		select {
		case event := <-events:
			payload, ok := want[event.Name]
			if !ok || !reflect.DeepEqual(event.Data, payload) {
				t.Fatalf("unexpected event: %+v", event)
			}
			delete(want, event.Name)
		case <-time.After(time.Second):
			t.Fatalf("missing Wails events: %v", want)
		}
	}
	payload, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"nodes", "discoveredNodes", "appState"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("missing frontend state field %q: %s", key, payload)
		}
	}
}
