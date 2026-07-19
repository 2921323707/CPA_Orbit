//go:build !windows

package main

type trayController struct{}

func newTray(func(), func()) *trayController { return &trayController{} }
func (*trayController) Start()               {}
func (*trayController) Stop()                {}
