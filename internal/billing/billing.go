// Package billing implements recurring invoice generation from contract
// billing cycles and time-entry rollups, plus overdue-invoice status
// automation. It is deliberately storage-only (no HTTP/hub dependency) so it
// can be called both from an HTTP handler (manual "generate now") and a
// background scheduler (cmd/server) without import cycles.
package billing

import (
	"errors"
	"fmt"
	"time"

	"gomeshcentral/internal/storage"
)

var ErrContractNotDue = errors.New("contract is not due for billing yet")

// billingCycleInterval returns how often a contract should be re-invoiced.
// "one_time" contracts are never auto-billed (nil interval => never due).
func billingCycleInterval(cycle string) (time.Duration, bool) {
	switch cycle {
	case "monthly":
		return 30 * 24 * time.Hour, true
	case "quarterly":
		return 90 * 24 * time.Hour, true
	case "annual":
		return 365 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

// IsContractDue reports whether a contract's billing cycle has elapsed since
// it was last invoiced (or since creation, if never invoiced).
func IsContractDue(contract storage.Contract, now time.Time) bool {
	if contract.Status != "active" {
		return false
	}
	interval, ok := billingCycleInterval(contract.BillingCycle)
	if !ok {
		return false
	}
	reference := contract.LastInvoicedAt
	if reference.IsZero() {
		reference = contract.CreatedAt
	}
	return now.Sub(reference) >= interval
}

// GenerateInvoiceForContract builds an invoice for a contract's recurring
// charge plus any unbilled billable time entries logged against the same
// client, marks those entries as invoiced, and stamps the contract's
// last-invoiced timestamp. force=true bypasses the IsContractDue check (used
// by the manual "Generate Invoice Now" button).
func GenerateInvoiceForContract(store storage.Store, contract storage.Contract, now time.Time, force bool) (storage.Invoice, error) {
	if !force && !IsContractDue(contract, now) {
		return storage.Invoice{}, ErrContractNotDue
	}

	var lineItems []storage.InvoiceLineItem
	if contract.RateAmount > 0 {
		lineItems = append(lineItems, storage.InvoiceLineItem{
			Description: fmt.Sprintf("%s (%s)", contract.Name, billingCycleLabel(contract.BillingCycle)),
			Quantity:    1,
			UnitPrice:   contract.RateAmount,
		})
	}

	unbilled := store.ListUnbilledTimeEntries(contract.OrgID, contract.ClientID)
	invoicedIDs := make([]string, 0, len(unbilled))
	for _, entry := range unbilled {
		hours := float64(entry.Minutes) / 60.0
		lineItems = append(lineItems, storage.InvoiceLineItem{
			Description: describeTimeEntry(entry),
			Quantity:    hours,
			UnitPrice:   contract.RateAmount,
		})
		invoicedIDs = append(invoicedIDs, entry.ID)
	}

	if len(lineItems) == 0 {
		return storage.Invoice{}, errors.New("nothing to bill: contract has no recurring rate and no unbilled time entries")
	}

	issueDate := now.UTC()
	invoice, err := store.CreateInvoice(storage.Invoice{
		OrgID:      contract.OrgID,
		ClientID:   contract.ClientID,
		ContractID: contract.ID,
		Status:     "draft",
		IssueDate:  issueDate,
		DueDate:    issueDate.AddDate(0, 0, 30),
		LineItems:  lineItems,
		Notes:      "Auto-generated from contract " + contract.Name,
	})
	if err != nil {
		return storage.Invoice{}, err
	}

	if err := store.MarkTimeEntriesInvoiced(invoicedIDs, invoice.ID); err != nil {
		return invoice, fmt.Errorf("invoice created but failed to mark time entries billed: %w", err)
	}
	if err := store.SetContractLastInvoiced(contract.ID, now.UTC()); err != nil {
		return invoice, fmt.Errorf("invoice created but failed to update contract last-invoiced date: %w", err)
	}
	return invoice, nil
}

func billingCycleLabel(cycle string) string {
	switch cycle {
	case "monthly":
		return "Monthly"
	case "quarterly":
		return "Quarterly"
	case "annual":
		return "Annual"
	default:
		return "One-Time"
	}
}

func describeTimeEntry(entry storage.TimeEntry) string {
	if entry.Description != "" {
		return entry.Description
	}
	return "Billable time"
}

// RunDueContracts generates invoices for every active contract whose billing
// cycle has elapsed. Used by the background scheduler; per-contract failures
// are returned but do not stop processing the rest.
func RunDueContracts(store storage.Store, orgID string, now time.Time) (generated int, errs []error) {
	for _, contract := range store.ListContracts(orgID, "") {
		if !IsContractDue(contract, now) {
			continue
		}
		if _, err := GenerateInvoiceForContract(store, contract, now, true); err != nil {
			errs = append(errs, fmt.Errorf("contract %s (%s): %w", contract.ID, contract.Name, err))
			continue
		}
		generated++
	}
	return generated, errs
}

// MarkOverdueInvoices flips any "sent" invoice past its due date to
// "overdue". Draft/paid/void invoices are left untouched.
func MarkOverdueInvoices(store storage.Store, orgID string, now time.Time) (updated int, errs []error) {
	for _, invoice := range store.ListInvoices(orgID, "") {
		if invoice.Status != "sent" {
			continue
		}
		if invoice.DueDate.IsZero() || !now.After(invoice.DueDate) {
			continue
		}
		invoice.Status = "overdue"
		if err := store.UpdateInvoice(invoice); err != nil {
			errs = append(errs, fmt.Errorf("invoice %s: %w", invoice.ID, err))
			continue
		}
		updated++
	}
	return updated, errs
}
