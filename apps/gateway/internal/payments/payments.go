// Package payments defines the provider-agnostic payment abstraction for INR
// wallet top-ups.
//
// The wallet service only knows the Provider interface: it can never be
// coupled to a specific payment gateway. New processors (Razorpay, Stripe,
// UPI) implement the interface and are wired in via the gateway constructor.
package payments

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Provider is implemented by any payment processor. Payments are verified
// before a wallet is credited.
type Provider interface {
	// CreateOrder initiates a top-up order and returns an order reference +
	// a client-facing checkout payload (e.g. a payment URL).
	CreateOrder(amountPaise int64, walletID string) (Order, error)
	// Verify checks a callback signature and confirms the order succeeded.
	// It is idempotent; repeated verification of the same order is allowed.
	Verify(orderID string, payload []byte) (bool, error)
}

// Order is a payment order returned by a provider.
type Order struct {
	ID         string `json:"id"`
	AmountPaise int64 `json:"amount_paise"`
	Status     string `json:"status"`
	Checkout   string `json:"checkout_url,omitempty"`
}

// ErrInvalidCallback indicates a callback signature/amount mismatch.
var ErrInvalidCallback = errors.New("payments: invalid callback")

// DemoProvider simulates a payment gateway (UPI/Razorpay-style) for the
// hackathon demo. Orders auto-approve after a short delay so the UI can show
// the full top-up lifecycle without real money.
type DemoProvider struct {
	mu      sync.Mutex
	next    int
	orders  map[string]Order
	env     map[string]string // envelope: amount_expected → wallet id
}

// NewDemoProvider returns a demo payment provider.
func NewDemoProvider() *DemoProvider {
	return &DemoProvider{orders: map[string]Order{}, env: map[string]string{}}
}

// CreateOrder records a demo order. The Checkout payload is a synthetic UPI
// link that would be handled by the client payment sheet in production.
func (d *DemoProvider) CreateOrder(amountPaise int64, walletID string) (Order, error) {
	if amountPaise <= 0 {
		return Order{}, errors.New("payments: order amount must be positive")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	id := fmt.Sprintf("ord_%d", d.next)
	o := Order{ID: id, AmountPaise: amountPaise, Status: "created",
		Checkout: fmt.Sprintf("upi://pay?pa=marketplace@okaxis&pn=AI+Marketplace&am=%.2f&tn=wallet+topup", float64(amountPaise)/100)}
	d.orders[id] = o
	d.env[id] = walletID
	return o, nil
}

// OrderAmount returns the stored amount for an order (0 if unknown). The
// wallet service calls this on verification so client-supplied amounts are
// never trusted.
func (d *DemoProvider) OrderAmount(orderID string) int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	o, ok := d.orders[orderID]
	if !ok {
		return 0
	}
	return o.AmountPaise
}

// Verify approves a demo order (idempotent). It validates that the supplied
// payload (simulated webhook body) declares the matching order id and amount.
func (d *DemoProvider) Verify(orderID string, payload []byte) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	o, ok := d.orders[orderID]
	if !ok {
		return false, fmt.Errorf("payments: unknown order %q", orderID)
	}
	if len(payload) == 0 {
		return false, ErrInvalidCallback
	}
	// Simulate webhook validation: require the order id echo with a fixed secret.
	if string(payload) != "webhook:"+orderID {
		return false, ErrInvalidCallback
	}
	// Simulate async settlement latency so the UI can show a "processing" state.
	time.Sleep(150 * time.Millisecond)
	if o.Status == "created" {
		o.Status = "paid"
		d.orders[orderID] = o
	}
	return true, nil
}