# Multi-Chain Wallet Demo

[![CI](https://github.com/OKaluzny/wallet-demo/actions/workflows/ci.yml/badge.svg)](https://github.com/OKaluzny/wallet-demo/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.21-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Networks](https://img.shields.io/badge/Networks-BTC%20%7C%20ETH%20%7C%20TRX-blue)]()

Демонстраційний проєкт програмного крипто-гаманця з підтримкою кількох мереж.
Реалізує ключові архітектурні патерни для генерації адрес, моніторингу блокчейну, підписання та надсилання транзакцій.

## Можливості

- **HD-деривація ключів** — BIP-32/BIP-44 з єдиного seed для трьох мереж
- **Криптографія secp256k1** — реальна еліптична крива для ETH, BTC, TRX
- **Виявлення реорганізацій** — трекінг хешів блоків, інвалідація pending-подій
- **Підтвердження транзакцій** — настроювана глибина (confirmation depth)
- **Ідемпотентність** — захист від дублювання транзакцій через ідемпотентні ключі
- **Retry з exponential backoff** — стійкість до мережевих збоїв
- **Абстракції зберігання** — інтерфейси для nonce, tx log, watched addresses
- **Конфігурація через ENV** — всі параметри винесені в `config`
- **CI/CD** — GitHub Actions (lint, test, build)

## Архітектура

```
┌──────────────────────────────────────────────────┐
│                   main.go                        │
│          (оркестрація та демо-потік)              │
└─────────┬──────────────┬──────────────┬──────────┘
          │              │              │
          ▼              ▼              ▼
┌─────────────┐  ┌──────────────┐  ┌──────────┐
│   wallet/   │  │  listener/   │  │   tx/    │
│  Generator  │  │ BlockListener│  │ Builder  │
│  Signer     │  │ Manager      │  │          │
└──────┬──────┘  └──────┬───────┘  └────┬─────┘
       │                │               │
  ┌────┴────┐     ┌─────┴─────┐    ┌────┴────┐
  │ ETH BTC │     │  Polling  │    │ Nonce   │
  │ TRX ... │     │ + Reorg   │    │ Retry   │
  └─────────┘     │ Detection │    │ Idempot.│
                  └───────────┘    └─────────┘
       │                │               │
       └────────────────┼───────────────┘
                        ▼
               ┌────────────────┐
               │    storage/    │
               │ NonceStore     │
               │ TxStore        │
               │ WatchStore     │
               └────────────────┘
```

## Структура проєкту

```
wallet-demo/
├── cmd/wallet-demo/
│   └── main.go                  # точка входу, демо-сценарій
├── internal/
│   ├── config/
│   │   └── config.go            # конфігурація з ENV та дефолтами
│   ├── listener/
│   │   ├── listener.go          # BlockListener, PollingListener, Manager
│   │   └── listener_test.go     # 8 тестів (reorg, confirmation, events)
│   ├── storage/
│   │   ├── store.go             # інтерфейси NonceStore, TxStore, WatchStore
│   │   └── memory.go            # in-memory реалізації (thread-safe)
│   ├── tx/
│   │   ├── builder.go           # Builder: nonce, fee, sign, broadcast, idempotency
│   │   └── builder_test.go      # 5 тестів (idempotency, nonce, fees)
│   └── wallet/
│       ├── wallet.go            # інтерфейси Generator, Signer, HSMSigner
│       ├── eth.go               # ETH генерація + підпис (EIP-155)
│       ├── btc.go               # BTC генерація + підпис (P2PKH)
│       ├── trx.go               # TRX генерація + підпис
│       └── wallet_test.go       # 10 тестів (формати, детермінованість)
├── pkg/models/
│   └── models.go                # Network, DerivedAddress, Transaction, BlockEvent
├── .github/workflows/ci.yml     # CI: lint + test + build
├── .golangci.yml                # конфігурація лінтера
├── Makefile                     # build, test, lint, clean
├── go.mod
└── go.sum
```

## Підтримувані мережі

| Мережа | Модель | Derivation Path | Формат адреси | Крива |
|--------|--------|-----------------|---------------|-------|
| BTC | UTXO | `m/44'/0'/0'/0/i` | Base58Check (`1...`) | secp256k1 |
| ETH | Account | `m/44'/60'/0'/0/i` | Hex (`0x...`) | secp256k1 |
| TRX | Account | `m/44'/195'/0'/0/i` | Base58Check (`T...`) | secp256k1 |

## Ключові патерни

### Мультимережева абстракція

```go
type Generator interface {
    Network() models.Network
    GenerateFromSeed(seed []byte, index uint32) (*DerivedAddress, error)
}
```

Кожна мережа — окрема реалізація. Додавання нової мережі = новий файл, без зміни існуючого коду.

### HSM-Ready підписання

```go
type Signer interface {
    Sign(ctx context.Context, tx *Transaction, privateKey []byte) (*Transaction, error)
}
```

У production замінюється на HSM (PKCS#11 / AWS CloudHSM / GCP Cloud KMS) без зміни бізнес-логіки.

### Block Listener з виявленням реорганізацій

- Polling-based listener з настроюваним інтервалом
- Трекінг хешів блоків для виявлення chain reorgs
- Pending events з промоцією до `Confirmed` після досягнення глибини
- Manager координує слухачів усіх мереж (fan-in патерн)
- Інтерфейс `BlockFetcher` для абстракції RPC-викликів

### Transaction Builder

- **Nonce management** — атомарний трекінг per address
- **Fee estimation** — per-network стратегії з конфігурації
- **Retry з exponential backoff** — `1s, 4s, 9s...`
- **Idempotency** — захист від дублювання через `IdempotencyKey`

### Абстракції зберігання

```go
type NonceStore interface {
    GetAndIncrement(address string) (uint64, error)
}

type TxStore interface {
    Get(idempotencyKey string) (*models.Transaction, error)
    Put(idempotencyKey string, tx *models.Transaction) error
}

type WatchStore interface {
    Add(address string) error
    Remove(address string) error
    List() ([]string, error)
    Contains(address string) (bool, error)
}
```

In-memory реалізації включені. У production — PostgreSQL, Redis тощо.

## Запуск

```bash
go mod tidy
go run cmd/wallet-demo/main.go
```

### Конфігурація через змінні середовища

| Змінна | Опис | За замовчуванням |
|--------|------|------------------|
| `ETH_POLL_INTERVAL` | Інтервал опитування ETH | `1s` |
| `BTC_POLL_INTERVAL` | Інтервал опитування BTC | `2s` |
| `TRX_POLL_INTERVAL` | Інтервал опитування TRX | `1s` |
| `BROADCAST_MAX_RETRIES` | Максимум повторів broadcast | `3` |
| `CONTEXT_TIMEOUT` | Таймаут контексту | `15s` |
| `ETH_CHAIN_ID` | Chain ID для EIP-155 | `1` |
| `BTC_MAINNET` | Mainnet чи testnet | `true` |

## Тестування

```bash
# всі тести з race detector
make test

# або напряму
go test ./... -race -count=1
```

23 тести покривають:
- Генерацію адрес (формат, детермінованість, різні seed/index)
- Підписання транзакцій для кожної мережі
- Ідемпотентність та nonce-менеджмент
- Polling listener (events, stop, watch/unwatch)
- Підтвердження транзакцій та виявлення реорганізацій
- Manager (fan-in, маршрутизація по мережах)

## Лінтинг

```bash
make lint
```

Налаштований `golangci-lint` з правилами: errcheck, govet, staticcheck, gocritic, revive, gosec.

## Що додати в production

- [ ] Реальні RPC-клієнти (go-ethereum, btcd, tron-sdk)
- [ ] UTXO selection для BTC (coin selection algorithms)
- [ ] EIP-1559 fee estimation для ETH
- [ ] HSM інтеграція (PKCS#11)
- [ ] Persistence (PostgreSQL для nonce, tx log, watched addresses)
- [ ] Metrics & tracing (Prometheus + OpenTelemetry)
- [ ] Rate limiting для RPC calls
- [ ] WebSocket listener як альтернатива polling

## Залежності

| Пакет | Призначення |
|-------|-------------|
| `btcsuite/btcd/btcec/v2` | Еліптична крива secp256k1 |
| `btcsuite/btcd/btcutil` | Base58Check кодування |
| `tyler-smith/go-bip32` | HD key derivation (BIP-32) |
| `tyler-smith/go-bip39` | Мнемоніки (BIP-39, у тестах) |
| `golang.org/x/crypto` | Keccak256, RIPEMD160 |

## Ліцензія

MIT
