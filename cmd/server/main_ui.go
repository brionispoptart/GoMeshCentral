//go:build windows || darwin
// +build windows darwin

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gomeshcentral/internal/config"
	"gomeshcentral/internal/httpapi"
	"gomeshcentral/internal/storage"

	"github.com/getlantern/systray"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	trayIcon := flag.String("tray-icon", "assets/icons/server/server.ico", "path to tray icon file (.ico on Windows)")
	flag.Parse()

	cfg := config.FromEnv()
	store, err := storage.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to initialize sqlite store: %v", err)
	}
	if err := store.ResetDeviceConnections(); err != nil {
		log.Fatalf("failed to reset device connections: %v", err)
	}

	bootstrapHash, err := bcrypt.GenerateFromPassword([]byte(cfg.BootstrapAdminPass), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash bootstrap admin password: %v", err)
	}
	if err := store.UpsertBootstrapAdmin(cfg.BootstrapAdminUser, string(bootstrapHash), storage.DefaultOrgID); err != nil {
		log.Fatalf("failed to upsert bootstrap admin: %v", err)
	}

	server := httpapi.NewServer(cfg, store)

	log.Printf("server listening on %s", cfg.ListenAddr)

	quit := make(chan struct{})
	var quitOnce sync.Once
	requestQuit := func() {
		quitOnce.Do(func() { close(quit) })
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()

	go runBillingScheduler(store, quit)

	go func() {
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("server shutdown failed: %v", err)
		}
	}()

	go func() {
		err := <-serverErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server failed: %v", err)
		}
		requestQuit()
		systray.Quit()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		requestQuit()
		systray.Quit()
	}()

	iconBytes, err := os.ReadFile(*trayIcon)
	if err != nil {
		log.Printf("tray icon not loaded (%s): %v", *trayIcon, err)
	}

	systray.Run(func() {
		if len(iconBytes) > 0 {
			systray.SetIcon(iconBytes)
		}
		systray.SetTitle("GoMeshCentral Server")
		systray.SetTooltip("GoMeshCentral Server running")

		status := systray.AddMenuItem("Status: Running", "Current server status")
		status.Disable()
		openWebItem := systray.AddMenuItem("Open Web UI", "Open GoMeshCentral Web Console in default browser")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Shutdown Server", "Stop GoMeshCentral server")

		webURL := resolveWebURL(cfg.ListenAddr)

		go func() {
			for range openWebItem.ClickedCh {
				openBrowser(webURL)
			}
		}()

		go func() {
			<-quitItem.ClickedCh
			requestQuit()
			systray.Quit()
		}()
	}, func() {
		requestQuit()
	})
}
