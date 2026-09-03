//go:build !windows && !linux

package main

func readMachineID() string { return "" }
func readSystemID() string  { return "" }
func readBoardID() string   { return "" }
