//go:build windows || darwin

package main

import (
	"time"

	"github.com/getlantern/systray"
)

func runAgentUI(iconBytes []byte, statusText <-chan string, requestStop func(), stop <-chan struct{}, unattended unattendedControls) {
	go func() {
		<-stop
		systray.Quit()
	}()

	systray.Run(func() {
		if len(iconBytes) > 0 {
			systray.SetIcon(iconBytes)
		}
		systray.SetTitle("GoMeshCentral Agent")
		systray.SetTooltip("GoMeshCentral Agent")

		statusItem := systray.AddMenuItem("Status: Starting", "Current agent status")
		statusItem.Disable()
		systray.AddSeparator()

		if unattended.supported {
			unattendedItem := systray.AddMenuItemCheckbox(
				"Unattended Access",
				"Install a background service so this device stays managed after logoff (requires admin)",
				unattended.installed(),
			)
			go func() {
				for range unattendedItem.ClickedCh {
					enable := !unattendedItem.Checked()
					if err := unattended.toggle(enable); err != nil {
						statusItem.SetTitle("Status: unattended toggle failed")
						continue
					}
					// The privileged child runs asynchronously (and the user may
					// decline the UAC prompt), so reconcile the checkmark against
					// the real service state shortly afterwards.
					go func() {
						for i := 0; i < 6; i++ {
							time.Sleep(2 * time.Second)
							if unattended.installed() {
								unattendedItem.Check()
							} else {
								unattendedItem.Uncheck()
							}
						}
					}()
				}
			}()
			systray.AddSeparator()
		}

		quitItem := systray.AddMenuItem("Shutdown Agent", "Stop GoMeshCentral agent")

		go func() {
			for s := range statusText {
				statusItem.SetTitle("Status: " + s)
			}
		}()

		go func() {
			<-quitItem.ClickedCh
			requestStop()
			systray.Quit()
		}()
	}, func() {
		requestStop()
	})
}

func requestUIQuit() {
	systray.Quit()
}
