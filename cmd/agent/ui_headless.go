//go:build !windows && !darwin

package main

import "log"

func runAgentUI(_ []byte, statusText <-chan string, requestStop func(), stop <-chan struct{}, _ unattendedControls) {
	go func() {
		for s := range statusText {
			log.Printf("status: %s", s)
		}
	}()

	<-stop
	requestStop()
}

func requestUIQuit() {
	// No-op in headless mode.
}
