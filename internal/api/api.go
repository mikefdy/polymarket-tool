package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/mikefdy/polymarket-tool/internal/config"
	"github.com/mikefdy/polymarket-tool/internal/types"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) SearchMarkets(query string) ([]types.Market, error) {
	// Use the proper public-search endpoint
	// Filter to only show active events
	searchURL := fmt.Sprintf("%s/public-search?q=%s&events_status=active&limit_per_type=50",
		c.cfg.GammaURL, url.QueryEscape(query))

	var searchResp types.SearchResponse
	if err := c.get(searchURL, &searchResp); err != nil {
		return nil, err
	}

	// Extract markets from events (events contain their markets)
	var markets []types.Market
	for _, event := range searchResp.Events {
		// Filter by start date to exclude very old events
		if event.StartDate != "" {
			startDate, err := time.Parse(time.RFC3339, event.StartDate)
			if err == nil && startDate.Before(time.Now().AddDate(0, -18, 0)) {
				continue // Skip events older than 18 months
			}
		}
		markets = append(markets, event.Markets...)
	}

	// Sort by volume (highest first)
	sort.Slice(markets, func(i, j int) bool {
		volI, _ := strconv.ParseFloat(markets[i].Volume, 64)
		volJ, _ := strconv.ParseFloat(markets[j].Volume, 64)
		return volI > volJ
	})

	return markets, nil
}

func (c *Client) GetEventBySlug(slug string) (*types.Event, error) {
	url := fmt.Sprintf("%s/events/slug/%s", c.cfg.GammaURL, slug)

	var event types.Event
	if err := c.get(url, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func (c *Client) GetOrderBook(tokenID string) (*types.OrderBook, error) {
	url := fmt.Sprintf("%s/book?token_id=%s", c.cfg.ClobURL, tokenID)

	var book types.OrderBook
	if err := c.get(url, &book); err != nil {
		return nil, err
	}
	return &book, nil
}

func (c *Client) GetRecentTrades(limit int) ([]types.Trade, error) {
	url := fmt.Sprintf("%s/trades?limit=%d", c.cfg.DataAPIURL, limit)

	var trades []types.Trade
	if err := c.get(url, &trades); err != nil {
		return nil, err
	}
	return trades, nil
}

func (c *Client) GetLeaderboard(limit int) ([]types.LeaderboardEntry, error) {
	url := fmt.Sprintf("%s/v1/leaderboard?limit=%d", c.cfg.DataAPIURL, limit)

	var entries []types.LeaderboardEntry
	if err := c.get(url, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *Client) GetUserActivity(address string, limit int) ([]types.UserActivity, error) {
	url := fmt.Sprintf("%s/activity?user=%s&limit=%d", c.cfg.DataAPIURL, address, limit)

	var activity []types.UserActivity
	if err := c.get(url, &activity); err != nil {
		return nil, err
	}
	return activity, nil
}

func (c *Client) get(url string, result interface{}) error {
	resp, err := c.http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
