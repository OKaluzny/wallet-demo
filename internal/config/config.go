package config

import (
	"math/big"
	"os"
	"strconv"
	"time"
)

// Config holds all configurable parameters for the wallet demo.
type Config struct {
	// Listener poll intervals per network
	ETHPollInterval time.Duration
	BTCPollInterval time.Duration
	TRXPollInterval time.Duration

	// Transaction builder
	BroadcastMaxRetries int
	ContextTimeout      time.Duration

	// Fee defaults (used when on-chain estimation is unavailable)
	ETHDefaultFee *big.Int
	BTCDefaultFee *big.Int
	TRXDefaultFee *big.Int

	// ETH chain ID
	ETHChainID int64

	// BTC network
	BTCMainnet bool
}

// Default returns a Config populated with default values.
func Default() Config {
	return Config{
		ETHPollInterval: 1 * time.Second,
		BTCPollInterval: 2 * time.Second,
		TRXPollInterval: 1 * time.Second,

		BroadcastMaxRetries: 3,
		ContextTimeout:      15 * time.Second,

		ETHDefaultFee: big.NewInt(21_000 * 20_000_000_000), // 21000 gas * 20 gwei
		BTCDefaultFee: big.NewInt(10_000),                   // 10000 satoshi
		TRXDefaultFee: big.NewInt(1_000_000),                // 1 TRX bandwidth

		ETHChainID: 1,
		BTCMainnet: true,
	}
}

// FromEnv returns a Config populated from environment variables,
// falling back to defaults for unset values.
func FromEnv() Config {
	cfg := Default()

	if v := os.Getenv("ETH_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ETHPollInterval = d
		}
	}
	if v := os.Getenv("BTC_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.BTCPollInterval = d
		}
	}
	if v := os.Getenv("TRX_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.TRXPollInterval = d
		}
	}
	if v := os.Getenv("BROADCAST_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BroadcastMaxRetries = n
		}
	}
	if v := os.Getenv("CONTEXT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ContextTimeout = d
		}
	}
	if v := os.Getenv("ETH_CHAIN_ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.ETHChainID = n
		}
	}
	if v := os.Getenv("BTC_MAINNET"); v == "false" {
		cfg.BTCMainnet = false
	}

	return cfg
}
