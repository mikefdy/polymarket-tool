package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GammaURL        string
	ClobURL         string
	ClobWsURL       string
	DataAPIURL      string
	MinTradeUSD     float64
	MinLiquidityRatio float64
	WebhookURL      string
	SearchQueries   []string
	PollIntervalMs  int
}

func Load() *Config {
	return &Config{
		GammaURL:        "https://gamma-api.polymarket.com",
		ClobURL:         "https://clob.polymarket.com",
		ClobWsURL:       "wss://ws-subscriptions-clob.polymarket.com/ws/market",
		DataAPIURL:      "https://data-api.polymarket.com",
		MinTradeUSD:     getEnvFloat("MIN_TRADE_USD", 1000),
		MinLiquidityRatio: getEnvFloat("MIN_LIQUIDITY_RATIO", 0.05),
		WebhookURL:      os.Getenv("WEBHOOK_URL"),
		SearchQueries:   getEnvSlice("SEARCH_QUERIES", []string{"trump", "russia", "china", "war", "election"}),
		PollIntervalMs:  getEnvInt("POLL_INTERVAL_MS", 30000),
	}
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvSlice(key string, def []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return def
}
