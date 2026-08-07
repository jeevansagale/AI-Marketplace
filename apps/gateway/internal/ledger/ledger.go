// Package ledger implements the immutable append-only marketplace ledger.
// Every wallet operation (reserve, debit, refund, top-up) writes an entry.
// Entries are never edited or deleted.
package ledger

import (
	"sync"
	"time"
)

// EntryKind classifies a ledger write.
type EntryKind string

const (
	KindReserve     EntryKind = "reserve"
	KindSettle      EntryKind = "settle"
	KindRefund      EntryKind = "refund"
	KindTopUp       EntryKind = "topup"
	KindReserveVoid EntryKind = "reserve_void"
)

// Entry is a single immutable ledger record.
type Entry struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	WalletID    string    `json:"wallet_id"`
	Kind        EntryKind `json:"kind"`
	Amount      int64     `json:"amount_paise"` // positive credit, negative debit (INR paise)
	Reservation string    `json:"reservation,omitempty"`
	ProviderID  string    `json:"provider_id,omitempty"`
	ModelID     string    `json:"model_id,omitempty"`
	Reference   string    `json:"reference,omitempty"`
	Status      string    `json:"status"`
}

// Ledger is a concurrency-safe immutable ledger.
type Ledger struct {
	mu      sync.RWMutex
	entries []Entry
	nextID  int
}

// New returns an empty ledger.
func New() *Ledger {
	return &Ledger{}
}

// Append records an entry and returns its immutable ID.
func (l *Ledger) Append(e Entry) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextID++
	id := "LE-" + time.Now().Format("20060102150405") + "-" + itoa(l.nextID)
	e.ID = id
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	l.entries = append(l.entries, e)
	return id
}

// List returns a copy of all entries, newest first.
func (l *Ledger) List() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// ByWallet returns entries for a wallet, newest first.
func (l *Ledger) ByWallet(walletID string) []Entry {
	var out []Entry
	for _, e := range l.List() {
		if e.WalletID == walletID {
			out = append(out, e)
		}
	}
	return out
}

// Len returns the number of entries (exported for tests and metrics).
func (l *Ledger) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}