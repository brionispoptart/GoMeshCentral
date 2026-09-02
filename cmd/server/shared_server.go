package main

import (
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gomeshcentral/internal/billing"
	"gomeshcentral/internal/storage"
)

// resolveWebURL converts a listen address to a URL
func resolveWebURL(listenAddr string) string {
	addr := strings.TrimSpace(listenAddr)
	if addr == "" {
		return "http://localhost:8080"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "http://localhost" + strings.TrimPrefix(addr, "0.0.0.0")
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return "http://" + addr
	}
	return addr
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		log.Printf("failed to open browser for %s: %v", url, err)
	}
}

// runBillingScheduler periodically generates invoices for contracts whose
// billing cycle has elapsed and flips overdue "sent" invoices to "overdue".
// Runs once at startup, then hourly, until quit closes.
func runBillingScheduler(store *storage.SQLiteStore, quit <-chan struct{}) {
	runBillingPass(store)
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runBillingPass(store)
		case <-quit:
			return
		}
	}
}

// runBillingPass executes one pass of the billing scheduler
func runBillingPass(store *storage.SQLiteStore) {
	now := time.Now().UTC()

	for _, org := range store.ListOrganizations() {
		generated, genErrs := billing.RunDueContracts(store, org.ID, now)
		if generated > 0 {
			log.Printf("billing scheduler: generated %d recurring invoice(s) for org %s", generated, org.ID)
		}
		for _, err := range genErrs {
			log.Printf("billing scheduler: invoice generation failed: %v", err)
		}

		overdue, overdueErrs := billing.MarkOverdueInvoices(store, org.ID, now)
		if overdue > 0 {
			log.Printf("billing scheduler: marked %d invoice(s) overdue for org %s", overdue, org.ID)
		}
		for _, err := range overdueErrs {
			log.Printf("billing scheduler: overdue check failed: %v", err)
		}
	}
}
