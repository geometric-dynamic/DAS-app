package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type AppState struct {
	Simulating       bool `json:"simulating"`
	HasExternalNodes bool `json:"hasExternalNodes"`
}

type FrontendState struct {
	Nodes           Nodes                     `json:"nodes"`
	DiscoveredNodes map[string]DiscoveredNode `json:"discoveredNodes"`
	AppState        AppState                  `json:"appState"`
}

// App struct
type App struct {
	wails         *application.App
	frontendReady bool
	readyMu       sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

func (a *App) MarkFrontendReady() {
	a.readyMu.Lock()
	alreadyReady := a.frontendReady
	a.frontendReady = true
	a.readyMu.Unlock()

	if alreadyReady {
		a.NewDataNotify()
		return
	}

	StartExternalAcquisition()
	a.NewDataNotify()
}

func (a *App) ConnectNode(nodeKey string) error {
	if strings.HasPrefix(strings.ToLower(nodeKey), "psurc-") {
		return psurcManager.ConnectNode(nodeKey)
	}
	return cm01Manager.ConnectNode(nodeKey)
}

func (a *App) GetFrontendState() FrontendState {
	return FrontendState{
		Nodes:           snapshotConnectedNodes(),
		DiscoveredNodes: snapshotAllDiscoveredNodes(),
		AppState:        a.GetAppState(),
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) LogPrintln(log string) (len int) {
	len, _ = fmt.Println(log)
	return len
}

func (a *App) StartSim(count int) {
	PauseExternalAcquisition()
	recorderManager.Stop()
	statsManager.Stop()
	StopSim()
	StartSim(count)
}

func (a *App) StopSim() {
	recorderManager.Stop()
	statsManager.Stop()
	StopSim()
	ResumeExternalAcquisition()
}

func (a *App) GetAppState() AppState {
	return AppState{
		Simulating:       IsSimulating(),
		HasExternalNodes: HasExternalNodes(),
	}
}

func (a *App) StartGlobalStats() {
	statsManager.Start()
}

func (a *App) StopGlobalStats() {
	statsManager.Stop()
}

func (a *App) StartRecording() error {
	if err := recorderManager.Start(); err != nil {
		return err
	}
	a.wails.Event.Emit("recording-state", true)
	return nil
}

func (a *App) StopRecording() {
	recorderManager.Stop()
	a.wails.Event.Emit("recording-state", false)
}

func (a *App) NewDataNotify() {
	discovered := snapshotAllDiscoveredNodes()
	a.wails.Event.Emit("new-data", snapshotConnectedNodes())
	a.wails.Event.Emit("discovered-nodes", discovered)
	a.wails.Event.Emit("app-state", a.GetAppState())
}

func snapshotAllDiscoveredNodes() map[string]DiscoveredNode {
	discovered := cm01Manager.SnapshotDiscoveredNodes()
	for key, node := range psurcManager.SnapshotDiscoveredNodes() {
		discovered[key] = node
	}
	return discovered
}
