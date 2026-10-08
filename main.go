package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS
var app *App

func main() {
	app = NewApp()
	app.wails = application.New(application.Options{
		Name: "DAS Console",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	app.wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "DAS Console",
		MinWidth:         600,
		MinHeight:        500,
		Width:            800,
		Height:           900,
		URL:              "/",
		BackgroundColour: application.NewRGB(21, 21, 23),
	})

	if err := app.wails.Run(); err != nil {
		println("Error:", err.Error())
	}
}
