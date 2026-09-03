//go:build linux

package main

import "os"

func readMachineID() string {
	return readIdentityFile("/etc/machine-id")
}

func readSystemID() string {
	return readIdentityFile("/sys/class/dmi/id/product_uuid")
}

func readBoardID() string {
	return readIdentityFile("/sys/class/dmi/id/board_serial")
}

func readIdentityFile(path string) string {
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(value)
}
