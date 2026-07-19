//go:build !windows

package main

func setStartOnLogin(string, bool) error { return nil }
