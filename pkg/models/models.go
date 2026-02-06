package models

import "math/big"

// Network represents a blockchain network
type Network string

// Supported blockchain networks.
const (
	NetworkBTC Network = "BTC"
	NetworkETH Network = "ETH"
	NetworkTRX Network = "TRX"
)

// DerivedAddress holds a generated address with its derivation path
type DerivedAddress struct {
	Network        Network `json:"network"`
	Address        string  `json:"address"`
	DerivationPath string  `json:"derivation_path"`
	PublicKey      string  `json:"public_key"`
}

// Transaction represents a generic blockchain transaction
type Transaction struct {
	Network   Network  `json:"network"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Amount    *big.Int `json:"amount"`
	Fee       *big.Int `json:"fee,omitempty"`
	Nonce     uint64   `json:"nonce,omitempty"`
	Data      []byte   `json:"data,omitempty"`
	Signed    bool     `json:"signed"`
	TxHash    string   `json:"tx_hash,omitempty"`
	RawSigned []byte   `json:"-"`
}

// BlockEvent represents an event detected by a block listener
type BlockEvent struct {
	Network     Network  `json:"network"`
	BlockNumber uint64   `json:"block_number"`
	TxHash      string   `json:"tx_hash"`
	From        string   `json:"from"`
	To          string   `json:"to"`
	Amount      *big.Int `json:"amount"`
	Confirmed   bool     `json:"confirmed"`
	Reorged     bool     `json:"reorged,omitempty"`
}
