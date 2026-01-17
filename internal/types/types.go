package types

type MarketEvent struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type Market struct {
	ID          string        `json:"id"`
	ConditionID string        `json:"conditionId"`
	Question    string        `json:"question"`
	Slug        string        `json:"slug"`
	Outcomes    string        `json:"outcomes"`
	ClobTokens  string        `json:"clobTokenIds"`
	Volume      string        `json:"volume"`
	Liquidity   string        `json:"liquidity"`
	Active      bool          `json:"active"`
	Closed      bool          `json:"closed"`
	CreatedAt   string        `json:"createdAt"`
	Events      []MarketEvent `json:"events"`
}

func (m *Market) EventSlug() string {
	if len(m.Events) > 0 && m.Events[0].Slug != "" {
		return m.Events[0].Slug
	}
	return m.Slug
}

func (m *Market) EventTitle() string {
	if len(m.Events) > 0 && m.Events[0].Title != "" {
		return m.Events[0].Title
	}
	return m.Question
}

type Event struct {
	ID        string   `json:"id"`
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Volume    float64  `json:"volume"`
	Liquidity float64  `json:"liquidity"`
	Markets   []Market `json:"markets"`
}

type Trade struct {
	ProxyWallet     string  `json:"proxyWallet"`
	Side            string  `json:"side"`
	Asset           string  `json:"asset"`
	ConditionID     string  `json:"conditionId"`
	Size            float64 `json:"size"`
	Price           float64 `json:"price"`
	Timestamp       int64   `json:"timestamp"`
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	Outcome         string  `json:"outcome"`
	Name            string  `json:"name"`
	Pseudonym       string  `json:"pseudonym"`
	TransactionHash string  `json:"transactionHash"`
}

type UserActivity struct {
	ProxyWallet     string  `json:"proxyWallet"`
	Timestamp       int64   `json:"timestamp"`
	ConditionID     string  `json:"conditionId"`
	Type            string  `json:"type"`
	Size            float64 `json:"size"`
	UsdcSize        float64 `json:"usdcSize"`
	Price           float64 `json:"price"`
	Asset           string  `json:"asset"`
	Side            string  `json:"side"`
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	EventSlug       string  `json:"eventSlug"`
	Outcome         string  `json:"outcome"`
	Name            string  `json:"name"`
	TransactionHash string  `json:"transactionHash"`
}

type LeaderboardEntry struct {
	Rank        string  `json:"rank"`
	ProxyWallet string  `json:"proxyWallet"`
	UserName    string  `json:"userName"`
	XUsername   string  `json:"xUsername"`
	Volume      float64 `json:"vol"`
	PnL         float64 `json:"pnl"`
}

type Whale struct {
	Address string  `json:"address"`
	Name    string  `json:"name"`
	PnL     float64 `json:"pnl"`
	Volume  float64 `json:"volume"`
	AddedAt string  `json:"addedAt"`
	Note    string  `json:"note,omitempty"`
}

type SavedMarket struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	AddedAt string `json:"addedAt"`
}

type WsMessage struct {
	EventType string `json:"event_type"`
	Market    string `json:"market"`
	AssetID   string `json:"asset_id"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Side      string `json:"side"`
	Timestamp string `json:"timestamp"`
}

type DetectedTrade struct {
	Market    *Market
	AssetID   string
	Side      string
	Price     float64
	Size      float64
	UsdValue  float64
	Timestamp string
	Reason    string
	Wallet    string
	Trader    string
}

type OrderBook struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}
