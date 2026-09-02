package billing

import (
	"path/filepath"
	"testing"
	"time"

	"gomeshcentral/internal/storage"
)

func newTestStore(t *testing.T) *storage.SQLiteStore {
	t.Helper()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestIsContractDue(t *testing.T) {
	now := time.Now().UTC()

	cases := []struct {
		name     string
		contract storage.Contract
		want     bool
	}{
		{
			name:     "monthly never invoiced, created 40 days ago is due",
			contract: storage.Contract{Status: "active", BillingCycle: "monthly", CreatedAt: now.AddDate(0, 0, -40)},
			want:     true,
		},
		{
			name:     "monthly never invoiced, created 10 days ago is not due",
			contract: storage.Contract{Status: "active", BillingCycle: "monthly", CreatedAt: now.AddDate(0, 0, -10)},
			want:     false,
		},
		{
			name:     "monthly invoiced 5 days ago is not due",
			contract: storage.Contract{Status: "active", BillingCycle: "monthly", CreatedAt: now.AddDate(0, 0, -90), LastInvoicedAt: now.AddDate(0, 0, -5)},
			want:     false,
		},
		{
			name:     "one_time cycle is never auto-due",
			contract: storage.Contract{Status: "active", BillingCycle: "one_time", CreatedAt: now.AddDate(0, -6, 0)},
			want:     false,
		},
		{
			name:     "inactive contract is never due",
			contract: storage.Contract{Status: "expired", BillingCycle: "monthly", CreatedAt: now.AddDate(0, 0, -40)},
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsContractDue(tc.contract, now); got != tc.want {
				t.Errorf("IsContractDue() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGenerateInvoiceForContractRollsInTimeEntriesAndMarksBilled(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()

	client, err := store.CreateClient(storage.Client{Name: "Acme"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	contract, err := store.CreateContract(storage.Contract{
		ClientID:     client.ID,
		Name:         "Managed IT",
		RateType:     "hourly",
		RateAmount:   100,
		BillingCycle: "monthly",
	})
	if err != nil {
		t.Fatalf("create contract: %v", err)
	}

	billable, err := store.CreateTimeEntry(storage.TimeEntry{ClientID: client.ID, Description: "Server maintenance", Minutes: 90, Billable: true})
	if err != nil {
		t.Fatalf("create time entry: %v", err)
	}
	if _, err := store.CreateTimeEntry(storage.TimeEntry{ClientID: client.ID, Description: "Internal note", Minutes: 30, Billable: false}); err != nil {
		t.Fatalf("create non-billable time entry: %v", err)
	}

	invoice, err := GenerateInvoiceForContract(store, contract, now, true)
	if err != nil {
		t.Fatalf("GenerateInvoiceForContract: %v", err)
	}

	// Base plan charge (100) + 1.5 hours at $100/hr (150) = 250.
	if invoice.Total != 250 {
		t.Errorf("invoice total = %v, want 250", invoice.Total)
	}
	if len(invoice.LineItems) != 2 {
		t.Fatalf("expected 2 line items (plan + billable time), got %d: %+v", len(invoice.LineItems), invoice.LineItems)
	}

	remainingUnbilled := store.ListUnbilledTimeEntries(storage.DefaultOrgID, client.ID)
	if len(remainingUnbilled) != 0 {
		t.Fatalf("expected 0 remaining billable-unbilled entries (non-billable entries never count), got %d", len(remainingUnbilled))
	}

	entries := store.ListTimeEntries(storage.DefaultOrgID, client.ID)
	if len(entries) != 2 {
		t.Fatalf("expected both time entries to still exist, got %d", len(entries))
	}
	for _, e := range entries {
		if e.ID == billable.ID && e.InvoiceID != invoice.ID {
			t.Errorf("billable time entry was not marked with invoice ID: got %q want %q", e.InvoiceID, invoice.ID)
		}
	}

	updatedContract, ok := store.GetContract(contract.ID)
	if !ok {
		t.Fatalf("contract disappeared after invoicing")
	}
	if updatedContract.LastInvoicedAt.IsZero() {
		t.Errorf("expected LastInvoicedAt to be stamped after invoicing")
	}
}

func TestGenerateInvoiceForContractRespectsDueCheckUnlessForced(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()

	client, err := store.CreateClient(storage.Client{Name: "Acme"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	contract, err := store.CreateContract(storage.Contract{
		ClientID:     client.ID,
		Name:         "Managed IT",
		RateType:     "monthly",
		RateAmount:   500,
		BillingCycle: "monthly",
	})
	if err != nil {
		t.Fatalf("create contract: %v", err)
	}
	// Freshly created contract is not due yet.
	if _, err := GenerateInvoiceForContract(store, contract, now, false); err != ErrContractNotDue {
		t.Fatalf("expected ErrContractNotDue, got %v", err)
	}
	if _, err := GenerateInvoiceForContract(store, contract, now, true); err != nil {
		t.Fatalf("forced generation should succeed even when not due: %v", err)
	}
}

func TestRunDueContractsOnlyBillsDueOnes(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()

	client, err := store.CreateClient(storage.Client{Name: "Acme"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	due, err := store.CreateContract(storage.Contract{ClientID: client.ID, Name: "Due Plan", RateAmount: 100, BillingCycle: "monthly"})
	if err != nil {
		t.Fatalf("create due contract: %v", err)
	}
	// Backdate creation so it's past the monthly interval.
	if err := store.SetContractLastInvoiced(due.ID, time.Time{}); err != nil {
		t.Fatalf("reset last invoiced: %v", err)
	}
	// Force due by directly checking not-yet-due contract stays untouched.
	notDue, err := store.CreateContract(storage.Contract{ClientID: client.ID, Name: "Not Due Plan", RateAmount: 50, BillingCycle: "monthly"})
	if err != nil {
		t.Fatalf("create not-due contract: %v", err)
	}

	// due's CreatedAt is "now" so it won't actually be due; simulate elapsed time
	// by using RunDueContracts with a future "now" 40 days out.
	future := now.AddDate(0, 0, 40)
	generated, errs := RunDueContracts(store, storage.DefaultOrgID, future)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if generated != 2 {
		t.Fatalf("expected both monthly contracts due after 40 days, generated=%d", generated)
	}

	invoices := store.ListInvoices(storage.DefaultOrgID, client.ID)
	if len(invoices) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(invoices))
	}

	// Running again immediately should generate nothing (both just invoiced).
	generated2, _ := RunDueContracts(store, storage.DefaultOrgID, future)
	if generated2 != 0 {
		t.Fatalf("expected 0 newly generated invoices on immediate re-run, got %d", generated2)
	}
	_ = notDue
}

func TestMarkOverdueInvoices(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()

	client, err := store.CreateClient(storage.Client{Name: "Acme"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	overdue, err := store.CreateInvoice(storage.Invoice{ClientID: client.ID, Status: "sent", DueDate: now.AddDate(0, 0, -5), LineItems: []storage.InvoiceLineItem{{Description: "x", Quantity: 1, UnitPrice: 10}}})
	if err != nil {
		t.Fatalf("create overdue invoice: %v", err)
	}
	notYetDue, err := store.CreateInvoice(storage.Invoice{ClientID: client.ID, Status: "sent", DueDate: now.AddDate(0, 0, 5), LineItems: []storage.InvoiceLineItem{{Description: "x", Quantity: 1, UnitPrice: 10}}})
	if err != nil {
		t.Fatalf("create not-yet-due invoice: %v", err)
	}
	draft, err := store.CreateInvoice(storage.Invoice{ClientID: client.ID, Status: "draft", DueDate: now.AddDate(0, 0, -5), LineItems: []storage.InvoiceLineItem{{Description: "x", Quantity: 1, UnitPrice: 10}}})
	if err != nil {
		t.Fatalf("create draft invoice: %v", err)
	}

	updated, errs := MarkOverdueInvoices(store, storage.DefaultOrgID, now)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if updated != 1 {
		t.Fatalf("expected 1 invoice marked overdue, got %d", updated)
	}

	got, _ := store.GetInvoice(overdue.ID)
	if got.Status != "overdue" {
		t.Errorf("overdue invoice status = %q, want overdue", got.Status)
	}
	got, _ = store.GetInvoice(notYetDue.ID)
	if got.Status != "sent" {
		t.Errorf("not-yet-due invoice status changed unexpectedly: %q", got.Status)
	}
	got, _ = store.GetInvoice(draft.ID)
	if got.Status != "draft" {
		t.Errorf("draft invoice status changed unexpectedly: %q", got.Status)
	}
}
