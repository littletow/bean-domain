package main

import (
	"bean-domain/service"
	"bean-domain/store"
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
)

//go:embed all:frontend/dist
var assets embed.FS

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {
	userDir, err := os.UserConfigDir() // 获取系统配置目录
	if err != nil {
		log.Fatal(err)
	}
	// 用户数据目录
	dbPath := filepath.Join(userDir, "sslchecker")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)
	log.Println("用户数据目录，", dbPath)

	ds := store.NewDataStore(dbPath)
	manager := &service.AppManager{}

	// 2. 创建服务实例并注入 db
	domainService := service.NewDomainService(ds, manager)

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	manager.App = application.New(application.Options{
		Name:        "BeanDomain",
		Description: "A domain certificate inspection tool",
		Services: []application.Service{
			application.NewService(domainService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		OnShutdown: func() {
			ds.Close()
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	manager.MainWindow = manager.App.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "豆子域名管家",
		Width:     1280, // 默认宽度
		Height:    960,  // 默认高度
		MinWidth:  1024, // 最小宽度，保证侧边栏和表格能看清
		MinHeight: 768,  // 最小高度

		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})

	manager.MainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		// Hide the window
		manager.MainWindow.Hide()
		// Cancel the event so it doesn't get destroyed
		e.Cancel()
	})

	systemTray := manager.App.SystemTray.New()

	// Use the template icon on macOS so the clock respects light/dark modes.
	if runtime.GOOS == "darwin" {
		systemTray.SetTemplateIcon(icons.SystrayMacTemplate)
	}

	// 2. 定义左键点击逻辑：显示并聚焦窗口
	systemTray.OnClick(func() {
		if manager.MainWindow != nil {
			manager.MainWindow.Show()
			manager.MainWindow.Focus()
		}
	})

	// 3. 定义右键菜单
	menu := manager.App.NewMenu()
	menu.Add("显示窗口").OnClick(func(ctx *application.Context) {
		manager.MainWindow.Show()
		manager.MainWindow.Focus()
	})
	menu.AddSeparator() // 分割线
	menu.Add("退出").OnClick(func(ctx *application.Context) {
		manager.App.Quit()
	})

	systemTray.SetMenu(menu)

	// Run the application. This blocks until the application has been exited.
	err = manager.App.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
