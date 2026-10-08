package main

import (
	"testing"
	"time"

	"github.com/karalabe/hid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestUDPOfflineCleanupNotifiesAndAllowsRediscovery(t *testing.T) {
	previousManager, previousApp := cm01Manager, app
	m := &CM01Manager{sessions: make(map[string]*cm01Session)}
	cm01Manager = m
	app = NewApp()
	app.wails = application.New(application.Options{Name: "DAS test"})
	t.Cleanup(func() { cm01Manager, app = previousManager, previousApp })

	stale, _ := m.touchSession("192.0.2.10", cm01TypeCode)
	stale.connected = true
	stale.lastSeen = time.Now().Add(-cm01OfflineTimeout)
	live, _ := m.touchSession("192.0.2.11", pdatxTypeCode)
	nodesMu.Lock()
	nodes[stale.nodeKey] = &Node{Source: "external"}
	nodesMu.Unlock()
	t.Cleanup(func() {
		nodesMu.Lock()
		delete(nodes, stale.nodeKey)
		nodesMu.Unlock()
	})

	states := make(chan AppState, 1)
	cancel := app.wails.Event.On("app-state", func(event *application.CustomEvent) {
		states <- event.Data.(AppState)
	})
	t.Cleanup(cancel)
	done := make(chan struct{})
	go func() { m.cleanupOffline(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("offline cleanup deadlocked while notifying the frontend")
	}
	select {
	case state := <-states:
		if state.HasExternalNodes {
			t.Fatal("disconnected node still blocks simulation")
		}
	case <-time.After(time.Second):
		t.Fatal("frontend did not receive the disconnected state")
	}
	if len(m.sessions) != 1 || m.sessions[live.ip] != live {
		t.Fatal("cleanup removed a live device or retained the offline device")
	}
	if _, ok := snapshotConnectedNodes()[stale.nodeKey]; ok {
		t.Fatal("offline node remains in the connected snapshot")
	}
	rediscovered, created := m.touchSession(stale.ip, stale.typeCode)
	if !created || rediscovered.connected || rediscovered.nodeKey != stale.nodeKey {
		t.Fatal("returning device was not rediscovered as available to connect")
	}
}

func TestHIDReadFailureDoesNotRemoveReconnectedDevice(t *testing.T) {
	const key = "psurc-reconnected"
	oldDevice, currentDevice := &hid.Device{}, &hid.Device{}
	session := &psurcSession{nodeKey: key, connected: true, device: currentDevice}
	m := &PSURCManager{sessions: map[string]*psurcSession{key: session}}
	node := &Node{Source: "external"}
	nodesMu.Lock()
	nodes[key] = node
	nodesMu.Unlock()
	t.Cleanup(func() {
		nodesMu.Lock()
		delete(nodes, key)
		nodesMu.Unlock()
	})

	// Closed handles return a read error without requiring physical hardware.
	m.readLoop(key, oldDevice)
	if !session.connected || session.device != currentDevice || snapshotConnectedNodes()[key] == nil {
		t.Fatal("old reader removed the reconnected device")
	}
	m.readLoop(key, currentDevice)
	if session.connected || session.device != nil || snapshotConnectedNodes()[key] != nil {
		t.Fatal("failed reader did not clear its own connection and node")
	}
}
