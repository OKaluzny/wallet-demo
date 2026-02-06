# Multi-Chain Wallet Demo

Демонстрационный проект программного крипто-кошелька с поддержкой нескольких сетей.
Показывает архитектурные паттерны для: генерации адресов, слушания блокчейна, подписания и отправки транзакций.

## Архитектура

```
┌──────────────────────────────────────────────────┐
│                   main.go                         │
│          (orchestration & demo flow)              │
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
  │ TRX ... │     │  WebSocket│    │ Retry   │
  └─────────┘     │  (plug.)  │    │ Idempot.│
                  └───────────┘    └─────────┘
```

## Ключевые паттерны

### 1. Мультисетевая абстракция
```go
type Generator interface {
    Network() models.Network
    GenerateFromSeed(seed []byte, index uint32) (*DerivedAddress, error)
}
```
Каждая сеть — отдельная реализация. Добавление новой сети = новый файл, без изменения существующего кода.

### 2. HSM-Ready архитектура
```go
type Signer interface {
    Sign(ctx context.Context, tx *Transaction, privateKey []byte) (*Transaction, error)
}

type HSMSigner interface {
    SignWithHSM(ctx context.Context, tx *Transaction, keyID string) (*Transaction, error)
}
```
Signer абстрагирует подписание. В production заменяется на HSM (PKCS#11 / AWS CloudHSM / GCP Cloud KMS) без изменения бизнес-логики.

### 3. Block Listener с fan-in
- Polling-based listener (универсальный, работает для любого JSON-RPC)
- Manager координирует слушателей всех сетей
- Fan-in паттерн: события всех сетей → один обработчик
- Легко заменить polling на WebSocket для конкретной сети

### 4. Transaction Builder
- **Nonce management** — трекинг nonce per address (account model)
- **Fee estimation** — per-network стратегии
- **Retry с exponential backoff** — устойчивость к сетевым сбоям
- **Idempotency** — защита от дублирования при retry

## Поддерживаемые сети

| Сеть | Модель | Derivation Path | Формат адреса |
|------|--------|-----------------|---------------|
| BTC  | UTXO   | m/44'/0'/0'/0/i | Base58Check (1...) |
| ETH  | Account| m/44'/60'/0'/0/i| Hex (0x...) |
| TRX  | Account| m/44'/195'/0'/0/i| Base58Check (T...) |

## Запуск

```bash
go mod tidy
go run cmd/wallet-demo/main.go
```

## Что добавить в production

- [ ] BIP-39 мнемоники (go-bip39)
- [ ] Полный BIP-32 HD derivation (go-bip32)
- [ ] HSM интеграция (PKCS#11)
- [ ] Реальные RPC-клиенты (go-ethereum, btcd, tron-sdk)
- [ ] UTXO selection для BTC (coin selection algorithms)
- [ ] EIP-1559 fee estimation для ETH
- [ ] Reorg detection в listener
- [ ] Persistence (PostgreSQL для nonce, tx log, watched addresses)
- [ ] Metrics & tracing (Prometheus + OpenTelemetry)
- [ ] Rate limiting для RPC calls
