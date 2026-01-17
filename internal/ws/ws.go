package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mikefdy/polymarket-tool/internal/config"
	"github.com/mikefdy/polymarket-tool/internal/types"
)

type TradeHandler func(msg types.WsMessage)

type Client struct {
	cfg             *config.Config
	conn            *websocket.Conn
	assetIDs        map[string]bool
	mu              sync.RWMutex
	onTrade         TradeHandler
	done            chan struct{}
	reconnectCount  int
	maxReconnects   int
}

func New(cfg *config.Config, onTrade TradeHandler) *Client {
	return &Client{
		cfg:           cfg,
		assetIDs:      make(map[string]bool),
		onTrade:       onTrade,
		done:          make(chan struct{}),
		maxReconnects: 10,
	}
}

func (c *Client) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.cfg.ClobWsURL, nil)
	if err != nil {
		return err
	}

	c.conn = conn
	c.reconnectCount = 0
	log.Println("[WS] Connected")

	c.subscribeAll()

	go c.readLoop()
	return nil
}

func (c *Client) readLoop() {
	defer func() {
		c.conn.Close()
		c.scheduleReconnect()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[WS] Read error: %v", err)
			return
		}

		var msgs []types.WsMessage
		if err := json.Unmarshal(message, &msgs); err != nil {
			continue
		}

		for _, msg := range msgs {
			if msg.EventType == "last_trade_price" {
				c.onTrade(msg)
			}
		}
	}
}

func (c *Client) scheduleReconnect() {
	if c.reconnectCount >= c.maxReconnects {
		log.Println("[WS] Max reconnect attempts reached")
		return
	}

	delay := time.Duration(1<<c.reconnectCount) * time.Second
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	c.reconnectCount++
	log.Printf("[WS] Reconnecting in %v...", delay)

	time.Sleep(delay)

	select {
	case <-c.done:
		return
	default:
		if err := c.Connect(); err != nil {
			log.Printf("[WS] Reconnect failed: %v", err)
			c.scheduleReconnect()
		}
	}
}

func (c *Client) Subscribe(assetIDs []string) {
	c.mu.Lock()
	for _, id := range assetIDs {
		c.assetIDs[id] = true
	}
	c.mu.Unlock()

	if c.conn != nil {
		c.sendSubscription(assetIDs)
	}
}

func (c *Client) subscribeAll() {
	c.mu.RLock()
	ids := make([]string, 0, len(c.assetIDs))
	for id := range c.assetIDs {
		ids = append(ids, id)
	}
	c.mu.RUnlock()

	if len(ids) > 0 {
		c.sendSubscription(ids)
	}
}

func (c *Client) sendSubscription(assetIDs []string) {
	msg := map[string]interface{}{
		"assets_ids": assetIDs,
		"type":       "market",
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		log.Printf("[WS] Subscribe error: %v", err)
		return
	}

	fmt.Printf("[WS] Subscribed to %d assets\n", len(assetIDs))
}

func (c *Client) Close() {
	close(c.done)
	if c.conn != nil {
		c.conn.Close()
	}
}
