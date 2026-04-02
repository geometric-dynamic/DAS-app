package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type AppState struct {
	Simulating       bool `json:"simulating"`
	HasExternalNodes bool `json:"hasExternalNodes"`
}

// App struct
type App struct {
	ctx           context.Context
	frontendReady bool
	readyMu       sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
	runtime.EventsEmit(a.ctx, "recording-state", true)
	return nil
}

func (a *App) StopRecording() {
	recorderManager.Stop()
	runtime.EventsEmit(a.ctx, "recording-state", false)
}

func (a *App) NewDataNotify() {
	runtime.EventsEmit(a.ctx, "new-data", nodes)
	runtime.EventsEmit(a.ctx, "app-state", a.GetAppState())
}
