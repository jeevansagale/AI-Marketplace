// Package wallet implements the INR prepaid wallet with session reservations.
//
// A reservation holds a portion of the available balance for the estimated
// cost of a session. When the session completes the actual cost is charged and
// the remainder is refunded to the available balance.
package wallet

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrInsufficientFunds is returned when a reserve or charge exceeds limits.
var ErrInsufficientFunds = errors.New("wallet: insufficient funds")

// TransactionKind classifies a wallet transaction.
type TransactionKind string

const (
	TxReserve     TransactionKind = "reserve"
	TxCharge      TransactionKind = "charge"
	TxRefund      TransactionKind = "refund"
	TxTopUp       TransactionKind = "topup"
	TxReserveVoid TransactionKind = "reserve_void"
)

// Transaction is a wallet entry in the user-facing history.
type Transaction struct {
	ID          string          `json:"id"`
	Timestamp   time.Time       `json:"timestamp"`
	Kind        TransactionKind `json:"kind"`
	Amount      int64           `json:"amount_paise"` // signed: -charge, +refund/topup
	ModelID     string          `json:"model_id,omitempty"`
	ProviderID  string          `json:"provider_id,omitempty"`
	Reservation string          `json:"reservation,omitempty"`
	Reference   string          `json:"reference,omitempty"`
}

// State is a point-in-time snapshot of a wallet.
type State struct {
	WalletID  string `json:"wallet_id"`
	Balance   int64  `json:"balance_paise"`   // total deposited
	Reserved  int64  `json:"reserved_paise"`  // held by open reservations
	Available int64  `json:"available_paise"` // spendable
}

// reservation is an in-flight hold on wallet funds.
type reservation struct {
	id     string
	amount int64
	ref    string
}

type account struct {
	balance       int64 // total
	reserved      int64
	tx            []Transaction
	reservations  map[string]*reservation
}

// Service manages all INR wallets with session reservation support.
type Service struct {
	mu       sync.RWMutex
	accounts map[string]*account
	nextID   int
}

// NewService creates a wallet service. Initial wallets may be seeded with
// name → starting balance in paise.
func NewService(seeds map[string]int64) *Service {
	s := &Service{accounts: map[string]*account{}}
	for id, bal := range seeds {
		s.accounts[id] = &account{balance: bal, reservations: map[string]*reservation{}}
	}
	return s
}

// Balance returns the current state of a wallet.
func (s *Service) Balance(walletID string) (State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[walletID]
	if !ok {
		return State{}, fmt.Errorf("wallet: unknown wallet %q", walletID)
	}
	return State{
		WalletID:  walletID,
		Balance:   a.balance,
		Reserved:  a.reserved,
		Available: a.balance - a.reserved,
	}, nil
}

// Reserve holds amountPaise for a session and returns a reservation ID.
func (s *Service) Reserve(walletID string, amountPaise int64, ref string) (string, error) {
	if amountPaise <= 0 {
		return "", errors.New("wallet: reserve amount must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[walletID]
	if !ok {
		return "", fmt.Errorf("wallet: unknown wallet %q", walletID)
	}
	if a.balance-a.reserved < amountPaise {
		return "", ErrInsufficientFunds
	}
	s.nextID++
	id := fmt.Sprintf("R-%d", s.nextID)
	a.reserved += amountPaise
	a.reservations[id] = &reservation{id: id, amount: amountPaise, ref: ref}
	a.tx = append(a.tx, Transaction{
		Timestamp: time.Now().UTC(), Kind: TxReserve, Amount: -amountPaise,
		Reservation: id, Reference: ref,
	})
	return id, nil
}

// Charge settles a reservation at the actual session cost, returning the
// refunded amount in paise (0 if the charge exactly matched).
func (s *Service) Charge(walletID, reservationID string, actualPaise int64) (int64, error) {
	if actualPaise < 0 {
		return 0, errors.New("wallet: charge amount must not be negative")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[walletID]
	if !ok {
		return 0, fmt.Errorf("wallet: unknown wallet %q", walletID)
	}
	r, ok := a.reservations[reservationID]
	if !ok {
		return 0, fmt.Errorf("wallet: unknown reservation %q", reservationID)
	}
	if actualPaise > r.amount {
		return 0, fmt.Errorf("wallet: charge %d exceeds reservation %d", actualPaise, r.amount)
	}
	delete(a.reservations, reservationID)
	a.reserved -= r.amount
	a.balance -= actualPaise
	refund := r.amount - actualPaise

	a.tx = append(a.tx, Transaction{
		Timestamp: time.Now().UTC(), Kind: TxCharge, Amount: -actualPaise,
		Reservation: reservationID,
	})
	if refund > 0 {
		a.tx = append(a.tx, Transaction{
			Timestamp: time.Now().UTC(), Kind: TxRefund, Amount: refund,
			Reservation: reservationID,
		})
	}
	return refund, nil
}

// Refund releases a reservation fully (used on cancellation/failure).
func (s *Service) Refund(walletID, reservationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[walletID]
	if !ok {
		return fmt.Errorf("wallet: unknown wallet %q", walletID)
	}
	r, ok := a.reservations[reservationID]
	if !ok {
		return fmt.Errorf("wallet: unknown reservation %q", reservationID)
	}
	delete(a.reservations, reservationID)
	a.reserved -= r.amount
	a.tx = append(a.tx, Transaction{
		Timestamp: time.Now().UTC(), Kind: TxReserveVoid, Amount: r.amount,
		Reservation: reservationID,
	})
	return nil
}

// TopUp credits a wallet after payment verification.
func (s *Service) TopUp(walletID string, amountPaise int64, ref string) error {
	if amountPaise <= 0 {
		return errors.New("wallet: top-up amount must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[walletID]
	if !ok {
		return fmt.Errorf("wallet: unknown wallet %q", walletID)
	}
	a.balance += amountPaise
	a.tx = append(a.tx, Transaction{
		Timestamp: time.Now().UTC(), Kind: TxTopUp, Amount: amountPaise, Reference: ref,
	})
	return nil
}

// Transactions returns the history for a wallet, newest first.
func (s *Service) Transactions(walletID string) ([]Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[walletID]
	if !ok {
		return nil, fmt.Errorf("wallet: unknown wallet %q", walletID)
	}
	out := make([]Transaction, len(a.tx))
	copy(out, a.tx)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// SeedBalance returns the demo wallet id used across the gateway.
const DemoWalletID = "wallet-demo"