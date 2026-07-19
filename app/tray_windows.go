//go:build windows

package main

import (
	_ "embed"
	"time"

	"github.com/getlantern/systray"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

type trayController struct {
	onShow  func()
	onQuit  func()
	ready   chan struct{}
	exited  chan struct{}
	started bool
}

func newTray(onShow, onQuit func()) *trayController {
	return &trayController{onShow: onShow, onQuit: onQuit, ready: make(chan struct{}), exited: make(chan struct{})}
}

func (t *trayController) Start() {
	if t == nil || t.started {
		return
	}
	t.started = true
	go systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("CPA Orbit")
		systray.SetTooltip("CPA Orbit · 订阅与价格监控")
		show := systray.AddMenuItem("显示 CPA Orbit", "打开桌面窗口")
		systray.AddSeparator()
		quit := systray.AddMenuItem("退出 CPA Orbit", "停止桌面应用")
		close(t.ready)
		go func() {
			for {
				select {
				case <-show.ClickedCh:
					if t.onShow != nil {
						t.onShow()
					}
				case <-quit.ClickedCh:
					if t.onQuit != nil {
						t.onQuit()
					}
				}
			}
		}()
	}, func() {
		close(t.exited)
	})
	select {
	case <-t.ready:
	case <-time.After(3 * time.Second):
	}
}

func (t *trayController) Stop() {
	if t == nil || !t.started {
		return
	}
	systray.Quit()
	select {
	case <-t.exited:
	case <-time.After(2 * time.Second):
	}
	t.started = false
}
