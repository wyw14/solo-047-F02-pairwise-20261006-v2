package landing

import (
	"sync"
	"time"
)

type IsolationReceipt struct {
	StationID   string    `json:"station_id"`
	CircuitID   string    `json:"circuit_id"`
	OperationID string    `json:"operation_id"`
	Isolated    bool      `json:"isolated"`
	ReceivedAt  time.Time `json:"received_at"`
}

type ReceiptRegistry struct {
	mu       sync.RWMutex
	receipts map[string]IsolationReceipt
}

func NewReceiptRegistry() *ReceiptRegistry {
	return &ReceiptRegistry{receipts: make(map[string]IsolationReceipt)}
}

func receiptKey(stationID, circuitID, operationID string) string {
	return stationID + "\x00" + circuitID + "\x00" + operationID
}

func (r *ReceiptRegistry) Record(receipt IsolationReceipt) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.receipts[receiptKey(receipt.StationID, receipt.CircuitID, receipt.OperationID)] = receipt
}

func (r *ReceiptRegistry) Match(stationID, circuitID, operationID string) (IsolationReceipt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	receipt, exists := r.receipts[receiptKey(stationID, circuitID, operationID)]
	return receipt, exists && receipt.Isolated
}

func (r *ReceiptRegistry) ForStation(stationID string) []IsolationReceipt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []IsolationReceipt
	for _, receipt := range r.receipts {
		if receipt.StationID == stationID {
			result = append(result, receipt)
		}
	}
	return result
}
