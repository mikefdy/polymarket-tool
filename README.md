# Polymarket Tool

Real-time detection of large and early trades on Polymarket. Track specific markets, monitor whale wallets, and get instant notifications.

## Installation

Download the latest binary for your platform from [Releases](https://github.com/mikefdy/polymarket-tool/releases):

**macOS (Apple Silicon):**
```bash
curl -L -o polymarket-tool https://github.com/mikefdy/polymarket-tool/releases/latest/download/polymarket-tool-darwin-arm64
chmod +x polymarket-tool
sudo mv polymarket-tool /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L -o polymarket-tool https://github.com/mikefdy/polymarket-tool/releases/latest/download/polymarket-tool-darwin-amd64
chmod +x polymarket-tool
sudo mv polymarket-tool /usr/local/bin/
```

**Linux:**
```bash
curl -L -o polymarket-tool https://github.com/mikefdy/polymarket-tool/releases/latest/download/polymarket-tool-linux
chmod +x polymarket-tool
sudo mv polymarket-tool /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri https://github.com/mikefdy/polymarket-tool/releases/latest/download/polymarket-tool.exe -OutFile polymarket-tool.exe
Move-Item polymarket-tool.exe C:\Windows\System32\
```

### Build from source

Requires Go 1.21+:

```bash
go build -o polymarket-tool .
```

## Quick Start

```bash
# Add a market to watch
polymarket-tool add-market https://polymarket.com/event/fed-decision-in-january

# Add top traders to whale list
polymarket-tool discover-whales top10

# Start tracking
polymarket-tool start
```

## Commands

### `start`

Starts the real-time tracker. Monitors all watched markets for fat trades via WebSocket.

```bash
# Basic
polymarket-tool start

# With custom thresholds
MIN_TRADE_USD=500 polymarket-tool start

# With Discord/Slack webhook
WEBHOOK_URL=https://discord.com/api/webhooks/... polymarket-tool start
```

### `markets [query]`

Search for markets and interactively add them to your watch list.

```bash
# Interactive search
polymarket-tool markets

# Search with query
polymarket-tool markets fed
polymarket-tool markets trump tariffs
```

### `add-market <url>`

Add a Polymarket event directly by URL or slug.

```bash
# By URL
polymarket-tool add-market https://polymarket.com/event/fed-decision-in-january

# By slug
polymarket-tool add-market fed-decision-in-january
```

### `fat-trades [min-usd]`

Scan historical trades for your saved markets and show fat trades.

```bash
# Use default threshold ($1000)
polymarket-tool fat-trades

# Custom threshold
polymarket-tool fat-trades 5000
polymarket-tool fat-trades 500
```

### `discover-whales [selection]`

Fetch the Polymarket leaderboard and add profitable traders to your whale list.

```bash
# Interactive mode
polymarket-tool discover-whales

# Add top N traders
polymarket-tool discover-whales top5
polymarket-tool discover-whales top20

# Add specific ranks
polymarket-tool discover-whales 1,2,3

# Add all untracked
polymarket-tool discover-whales all
```

### `whale-trades [name] [limit]`

View recent trades for tracked whales.

```bash
# All whales (default 20 trades each)
polymarket-tool whale-trades

# Specific whale by name
polymarket-tool whale-trades beachboy4

# With custom limit
polymarket-tool whale-trades beachboy4 50

# By index (from list)
polymarket-tool whale-trades 1
```

### `list <type>`

View and manage tracked whales and markets.

```bash
# List tracked whales
polymarket-tool list whales

# List saved markets
polymarket-tool list markets

# Clear all saved markets
polymarket-tool list clear-markets

# Clear all tracked whales
polymarket-tool list clear-whales

# Remove a whale
polymarket-tool list remove-whale 0x123...

# Remove a market
polymarket-tool list remove-market fed-decision-in-january
```

## Configuration

Set via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MIN_TRADE_USD` | 1000 | Minimum trade value to trigger alert |
| `MIN_LIQUIDITY_RATIO` | 0.05 | Min trade size as % of orderbook (5%) |
| `WEBHOOK_URL` | - | Discord/Slack webhook for notifications |
| `SEARCH_QUERIES` | trump,russia,china,war,election | Comma-separated market search terms |
| `POLL_INTERVAL_MS` | 30000 | Market list refresh interval (ms) |

## Detection Criteria

A trade triggers an alert if ANY of these conditions are met:

1. **Large trade** - Value ≥ `MIN_TRADE_USD`
2. **High liquidity ratio** - Trade size ≥ `MIN_LIQUIDITY_RATIO` of orderbook
3. **Early market** - Market < 24h old AND trade ≥ 50% of `MIN_TRADE_USD`
4. **Whale trade** - Trader is in your whale list (any size)

## Data Storage

Tracked whales and markets are stored in `data/`:

```
data/
├── whales.json    # Wallet addresses, names, PnL, volume
└── markets.json   # Market slugs and titles
```

Edit these files directly to add/remove entries manually.

## Notifications

### Console Output

```
============================================================
🐋 FAT TRADE DETECTED
Market: Fed decreases interest rates by 25 bps?
Outcome: Yes
Side: BUY
Size: 1000.00 @ 0.8500
Value: $850.00
Reason: Large trade: $850.00 | 🐋 Whale: beachboy4
URL: https://polymarket.com/event/fed-decision-in-january
Time: 2026-01-17T19:57:06Z
============================================================
```

### Webhook (Discord/Slack)

Set `WEBHOOK_URL` to receive rich embed notifications with trade details.

## Examples

### Track a specific event

```bash
polymarket-tool add-market https://polymarket.com/event/trump-tariffs-china
polymarket-tool start
```

### Monitor whale activity on Fed decisions

```bash
polymarket-tool add-market https://polymarket.com/event/fed-decision-in-january
polymarket-tool discover-whales top10
MIN_TRADE_USD=100 polymarket-tool start
```

### Low-threshold monitoring with Discord alerts

```bash
MIN_TRADE_USD=50 \
MIN_LIQUIDITY_RATIO=0.02 \
WEBHOOK_URL=https://discord.com/api/webhooks/xxx/yyy \
polymarket-tool start
```

## Building from Source

Requires Go 1.21+:

```bash
# Build for current platform
go build -o polymarket-tool .

# Cross-compile for other platforms
GOOS=linux GOARCH=amd64 go build -o polymarket-tool-linux .
GOOS=darwin GOARCH=arm64 go build -o polymarket-tool-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o polymarket-tool.exe .
```

## API Sources

- **Gamma API** - Market discovery and metadata
- **CLOB API** - Orderbook and pricing
- **Data API** - Historical trades and leaderboard
- **WebSocket** - Real-time trade stream
