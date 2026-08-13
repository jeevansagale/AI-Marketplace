// Package settlement pays providers out of collected INR wallet funds.
//
// The engine is interface-driven: a future implementation may settle via a
// blockchain adapter (e.g. Algorand). The MVP settles internally by recording
// a per-provider earnings balance in the ledger.
package settlement

import (
	"sync"

	"github.com/x402-marketplace/gateway/internal/ledger"
)

// Driver settles provider earnings. A future Algorand adapter implements this
// interface without touching the request path.
type Driver interface {
	// Settle records a provider payout. Returns a settlement reference.
	Settle(providerID string, amountPaise int64, ref string) error
}

// InternalDriver is the MVP settlement driver: it records earnings per
// provider in the immutable ledger. No blockchain is involved.
type InternalDriver struct {
	mu     sync.Mutex
	ledger *ledger.Ledger
}

// NewInternalDriver wires the internal settlement driver to a ledger.
func NewInternalDriver(l *ledger.Ledger) *InternalDriver {
	return &InternalDriver{ledger: l}
}

// Settle records a settlement entry in the ledger for a provider payout.
func (d *InternalDriver) Settle(providerID string, amountPaise int64, ref string) error {
	if amountPaise <= 0 {
		return nil // nothing to settle
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ledger.Append(ledger.Entry{
		Kind:       ledger.KindSettle, // provider payout credited to earnings
		WalletID:   "settlement",
		Amount:     amountPaise,
		ProviderID: providerID,
		Reference:  ref,
		Status:     "settled",
	})
	return nil
}
