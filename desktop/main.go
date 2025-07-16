package main

import (
	"embed"
	_ "embed"
	"time"
	"github.com/wailsapp/wails/v3/pkg/events"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/icons"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
     app := application.New(application.Options{
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Name:        "SimpleVPN",
		Description: "A simple VPN client",
		Services: []application.Service{
			application.NewService(&GreetService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		
		
	}) 

		systemTray := app.SystemTray.New()


	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Width:         500,
		Height:        500,
		Name:          "SimpleVPN",
		Frameless:     true,
		AlwaysOnTop:   true,
		Hidden:        false,
		DisableResize: false,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: false,
		},
		KeyBindings: map[string]func(window *application.WebviewWindow){
			"F12": func(window *application.WebviewWindow) {
				systemTray.OpenMenu()
			},
		},
	})

	myMenu := app.NewMenu()
	myMenu.Add("Wails").SetEnabled(false)
	myMenu.Add("Hidden").SetHidden(true)

	myMenu.Add("Hello World!").OnClick(func(ctx *application.Context) {
		println("Hello World!")
		q := application.QuestionDialog().SetTitle("Ready?").SetMessage("Are you feeling ready?")
		q.AddButton("Yes").OnClick(func() {
			println("Awesome!")
		})
		q.AddButton("No").SetAsDefault().OnClick(func() {
			println("Boo!")
		})
		q.Show()
	})
	subMenu := myMenu.AddSubmenu("Submenu")
	subMenu.Add("Click me!").OnClick(func(ctx *application.Context) {
		ctx.ClickedMenuItem().SetLabel("Clicked!")
	})
	myMenu.AddSeparator()
	myMenu.AddCheckbox("Checked", true).OnClick(func(ctx *application.Context) {
		println("Checked: ", ctx.ClickedMenuItem().Checked())
		application.InfoDialog().SetTitle("Hello World!").SetMessage("Hello World!").Show()
	})
	myMenu.Add("Enabled").OnClick(func(ctx *application.Context) {
		println("Click me!")
		ctx.ClickedMenuItem().SetLabel("Disabled!").SetEnabled(false)
	})
	myMenu.AddSeparator()
	// Callbacks can be shared. This is useful for radio groups
	radioCallback := func(ctx *application.Context) {
		menuItem := ctx.ClickedMenuItem()
		menuItem.SetLabel(menuItem.Label() + "!")
	}

	// Radio groups are created implicitly by placing radio items next to each other in a menu
	myMenu.AddRadio("Radio 1", true).OnClick(radioCallback)
	myMenu.AddRadio("Radio 2", false).OnClick(radioCallback)
	myMenu.AddRadio("Radio 3", false).OnClick(radioCallback)
	myMenu.AddSeparator()
	myMenu.Add("Hide System tray for 3 seconds...").OnClick(func(ctx *application.Context) {
		systemTray.Hide()
		time.Sleep(3 * time.Second)
		systemTray.Show()
	})
	myMenu.AddSeparator()
	myMenu.Add("Quit").OnClick(func(ctx *application.Context) {
		app.Quit()
	})


	// Register a hook to hide the window when the window is closing
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		// Hide the window
		window.Hide()
		// Cancel the event so it doesn't get destroyed
		e.Cancel()
	})

	if runtime.GOOS == "darwin" {
		systemTray.SetTemplateIcon(icons.SystrayMacTemplate)
	}

		systemTray.SetMenu(myMenu)

	systemTray.AttachWindow(window).WindowOffset(5)

	// window.OnClose(func(ctx *application.Context) bool {
	// 	window.Hide()
	// 	return false // Prevent the app from quitting
	// })

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	app.Run()

}