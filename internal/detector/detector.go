package detector

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mikefdy/polymarket-tool/internal/api"
	"github.com/mikefdy/polymarket-tool/internal/config"
	"github.com/mikefdy/polymarket-tool/internal/types"
)

type DetectionHandler func(detection types.DetectedTrade)

type Detector struct {
	cfg            *config.Config
	api            *api.Client
	markets        map[string]*types.Market
	assetToMarket  map[string]*types.Market
	liquidityCache map[string]liquidityEntry
	seenTxHashes   map[string]bool
	whaleAddresses map[string]bool
	whaleNames     map[string]string
	onDetection    DetectionHandler
	mu             sync.RWMutex
	cacheMu        sync.RWMutex
}

type liquidityEntry struct {
	value     float64
	timestamp time.Time
}

func New(cfg *config.Config, apiClient *api.Client, onDetection DetectionHandler) *Detector {
	return &Detector{
		cfg:            cfg,
		api:            apiClient,
		markets:        make(map[string]*types.Market),
		assetToMarket:  make(map[string]*types.Market),
		liquidityCache: make(map[string]liquidityEntry),
		seenTxHashes:   make(map[string]bool),
		whaleAddresses: make(map[string]bool),
		whaleNames:     make(map[string]string),
		onDetection:    onDetection,
	}
}

func (d *Detector) SetWhales(whales []types.Whale) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.whaleAddresses = make(map[string]bool)
	d.whaleNames = make(map[string]string)

	for _, w := range whales {
		addr := strings.ToLower(w.Address)
		d.whaleAddresses[addr] = true
		d.whaleNames[addr] = w.Name
	}
}

func (d *Detector) AddMarkets(markets []types.Market) []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var assetIDs []string
	for i := range markets {
		m := &markets[i]
		d.markets[m.ConditionID] = m

		tokenIDs := parseTokenIDs(m.ClobTokens)
		for _, tokenID := range tokenIDs {
			d.assetToMarket[tokenID] = m
			assetIDs = append(assetIDs, tokenID)
		}
	}
	return assetIDs
}

func (d *Detector) GetWatchedConditionIDs() map[string]bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ids := make(map[string]bool)
	for id := range d.markets {
		ids[id] = true
	}
	return ids
}

func (d *Detector) ProcessWsTrade(msg types.WsMessage) {
	d.mu.RLock()
	market := d.assetToMarket[msg.AssetID]
	d.mu.RUnlock()

	if market == nil {
		return
	}

	price, _ := strconv.ParseFloat(msg.Price, 64)
	size, _ := strconv.ParseFloat(msg.Size, 64)
	if size == 0 {
		return
	}

	usdValue := price * size
	reasons := d.checkDetectionCriteria(market, msg.AssetID, usdValue, "")

	if len(reasons) > 0 {
		d.onDetection(types.DetectedTrade{
			Market:    market,
			AssetID:   msg.AssetID,
			Side:      msg.Side,
			Price:     price,
			Size:      size,
			UsdValue:  usdValue,
			Timestamp: msg.Timestamp,
			Reason:    strings.Join(reasons, " | "),
		})
	}
}

func (d *Detector) ProcessHistoricalTrade(trade types.Trade) bool {
	if d.seenTxHashes[trade.TransactionHash] {
		return false
	}
	d.seenTxHashes[trade.TransactionHash] = true

	watchedIDs := d.GetWatchedConditionIDs()
	if !watchedIDs[trade.ConditionID] {
		return false
	}

	d.mu.RLock()
	market := d.markets[trade.ConditionID]
	d.mu.RUnlock()

	if market == nil {
		return false
	}

	usdValue := trade.Price * trade.Size
	reasons := d.checkDetectionCriteria(market, trade.Asset, usdValue, trade.ProxyWallet)

	if len(reasons) > 0 {
		trader := trade.Name
		if trader == "" {
			trader = trade.Pseudonym
		}
		d.onDetection(types.DetectedTrade{
			Market:    market,
			AssetID:   trade.Asset,
			Side:      strings.ToLower(trade.Side),
			Price:     trade.Price,
			Size:      trade.Size,
			UsdValue:  usdValue,
			Timestamp: strconv.FormatInt(trade.Timestamp*1000, 10),
			Reason:    "[HISTORICAL] " + strings.Join(reasons, " | "),
			Wallet:    trade.ProxyWallet,
			Trader:    trader,
		})
		return true
	}
	return false
}

func (d *Detector) checkDetectionCriteria(market *types.Market, assetID string, usdValue float64, wallet string) []string {
	var reasons []string

	if usdValue >= d.cfg.MinTradeUSD {
		reasons = append(reasons, formatUSD("Large trade: ", usdValue))
	}

	liquidity := d.getLiquidity(assetID)
	if liquidity > 0 {
		ratio := usdValue / liquidity
		if ratio >= d.cfg.MinLiquidityRatio {
			reasons = append(reasons, formatPercent(ratio*100)+"% of book liquidity")
		}
	}

	if market.CreatedAt != "" {
		createdAt, err := time.Parse(time.RFC3339, market.CreatedAt)
		if err == nil {
			marketAge := time.Since(createdAt)
			if marketAge < 24*time.Hour && usdValue >= d.cfg.MinTradeUSD/2 {
				reasons = append(reasons, "Early market (<24h old)")
			}
		}
	}

	if wallet != "" {
		d.mu.RLock()
		isWhale := d.whaleAddresses[strings.ToLower(wallet)]
		whaleName := d.whaleNames[strings.ToLower(wallet)]
		d.mu.RUnlock()

		if isWhale {
			if whaleName == "" {
				whaleName = wallet[:10]
			}
			reasons = append(reasons, "🐋 Whale: "+whaleName)
		}

		if isWhale && len(reasons) == 1 {
			return reasons
		}
	}

	return reasons
}

func (d *Detector) getLiquidity(assetID string) float64 {
	d.cacheMu.RLock()
	entry, ok := d.liquidityCache[assetID]
	d.cacheMu.RUnlock()

	if ok && time.Since(entry.timestamp) < time.Minute {
		return entry.value
	}

	book, err := d.api.GetOrderBook(assetID)
	if err != nil {
		return 0
	}

	var liq float64
	for _, bid := range book.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			size, _ := strconv.ParseFloat(bid[1], 64)
			liq += price * size
		}
	}
	for _, ask := range book.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			size, _ := strconv.ParseFloat(ask[1], 64)
			liq += price * size
		}
	}

	d.cacheMu.Lock()
	d.liquidityCache[assetID] = liquidityEntry{value: liq, timestamp: time.Now()}
	d.cacheMu.Unlock()

	return liq
}

func parseTokenIDs(jsonStr string) []string {
	var ids []string
	json.Unmarshal([]byte(jsonStr), &ids)
	return ids
}

func formatUSD(prefix string, value float64) string {
	if value >= 1_000_000 {
		return prefix + "$" + strconv.FormatFloat(value/1_000_000, 'f', 2, 64) + "M"
	}
	if value >= 1_000 {
		return prefix + "$" + strconv.FormatFloat(value/1_000, 'f', 1, 64) + "K"
	}
	return prefix + "$" + strconv.FormatFloat(value, 'f', 2, 64)
}

func formatPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}
