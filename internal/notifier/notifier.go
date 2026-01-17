package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikefdy/polymarket-tool/internal/config"
	"github.com/mikefdy/polymarket-tool/internal/types"
)

type Notifier struct {
	cfg  *config.Config
	http *http.Client
}

func New(cfg *config.Config) *Notifier {
	return &Notifier{
		cfg:  cfg,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *Notifier) Notify(detection types.DetectedTrade) {
	n.printConsole(detection)

	if n.cfg.WebhookURL != "" {
		n.sendWebhook(detection)
	}
}

func (n *Notifier) printConsole(d types.DetectedTrade) {
	outcome := getOutcome(d.Market, d.AssetID)
	ts := parseTimestamp(d.Timestamp)

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("🐋 FAT TRADE DETECTED")
	fmt.Printf("Market: %s\n", d.Market.Question)
	fmt.Printf("Outcome: %s\n", outcome)
	fmt.Printf("Side: %s\n", strings.ToUpper(d.Side))
	fmt.Printf("Size: %.2f @ %.4f\n", d.Size, d.Price)
	fmt.Printf("Value: $%.2f\n", d.UsdValue)
	if d.Trader != "" {
		fmt.Printf("Trader: %s\n", d.Trader)
	}
	if d.Wallet != "" {
		fmt.Printf("Wallet: %s\n", d.Wallet)
	}
	fmt.Printf("Reason: %s\n", d.Reason)
	fmt.Printf("URL: https://polymarket.com/event/%s\n", d.Market.Slug)
	fmt.Printf("Time: %s\n", ts.Format(time.RFC3339))
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
}

func (n *Notifier) sendWebhook(d types.DetectedTrade) {
	outcome := getOutcome(d.Market, d.AssetID)
	ts := parseTimestamp(d.Timestamp)

	color := 0x00ff00 // green for buy
	if strings.ToLower(d.Side) == "sell" {
		color = 0xff0000 // red for sell
	}

	payload := map[string]interface{}{
		"content": "**🐋 FAT TRADE DETECTED**",
		"embeds": []map[string]interface{}{
			{
				"title": d.Market.Question,
				"url":   fmt.Sprintf("https://polymarket.com/event/%s", d.Market.Slug),
				"color": color,
				"fields": buildWebhookFields(d, outcome),
				"timestamp": ts.Format(time.RFC3339),
			},
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := n.http.Post(n.cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Webhook] Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[Webhook] Failed: %d", resp.StatusCode)
	}
}

func buildWebhookFields(d types.DetectedTrade, outcome string) []map[string]interface{} {
	fields := []map[string]interface{}{
		{"name": "Outcome", "value": outcome, "inline": true},
		{"name": "Side", "value": strings.ToUpper(d.Side), "inline": true},
		{"name": "Value", "value": fmt.Sprintf("$%.2f", d.UsdValue), "inline": true},
		{"name": "Size", "value": fmt.Sprintf("%.2f", d.Size), "inline": true},
		{"name": "Price", "value": fmt.Sprintf("%.4f", d.Price), "inline": true},
	}
	if d.Trader != "" {
		fields = append(fields, map[string]interface{}{"name": "Trader", "value": d.Trader, "inline": true})
	}
	if d.Wallet != "" {
		wallet := d.Wallet
		if len(wallet) > 16 {
			wallet = wallet[:16] + "..."
		}
		fields = append(fields, map[string]interface{}{"name": "Wallet", "value": wallet, "inline": true})
	}
	fields = append(fields, map[string]interface{}{"name": "Reason", "value": d.Reason, "inline": false})
	return fields
}

func getOutcome(market *types.Market, assetID string) string {
	var outcomes []string
	var tokenIDs []string

	json.Unmarshal([]byte(market.Outcomes), &outcomes)
	json.Unmarshal([]byte(market.ClobTokens), &tokenIDs)

	for i, tokenID := range tokenIDs {
		if tokenID == assetID && i < len(outcomes) {
			return outcomes[i]
		}
	}
	return "Unknown"
}

func parseTimestamp(ts string) time.Time {
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.UnixMilli(ms)
}
