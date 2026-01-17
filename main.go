package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mikefdy/polymarket-tool/internal/api"
	"github.com/mikefdy/polymarket-tool/internal/config"
	"github.com/mikefdy/polymarket-tool/internal/detector"
	"github.com/mikefdy/polymarket-tool/internal/notifier"
	"github.com/mikefdy/polymarket-tool/internal/storage"
	"github.com/mikefdy/polymarket-tool/internal/types"
	"github.com/mikefdy/polymarket-tool/internal/ws"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "start":
		cmdStart()
	case "add-market":
		cmdAddMarket(args)
	case "discover-whales":
		cmdDiscoverWhales(args)
	case "whale-trades":
		cmdWhaleTrades(args)
	case "list":
		cmdList(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Polymarket Tool

Usage:
  polymarket-tool <command> [arguments]

Commands:
  start                Start the tracker
  add-market <url>     Add a market by URL or slug
  discover-whales      Discover and add whales from leaderboard
  whale-trades [name]  View recent trades for tracked whales
  list <type>          List whales or markets
  help                 Show this help

Examples:
  polymarket-tool start
  polymarket-tool add-market https://polymarket.com/event/fed-decision
  polymarket-tool discover-whales top10
  polymarket-tool whale-trades beachboy4
  polymarket-tool list whales`)
}

// ============= START COMMAND =============

func cmdStart() {
	cfg := config.Load()

	fmt.Println("Polymarket Tool")
	fmt.Println("===============")
	fmt.Printf("Min trade: $%.0f\n", cfg.MinTradeUSD)
	fmt.Printf("Min liquidity ratio: %.0f%%\n", cfg.MinLiquidityRatio*100)
	fmt.Printf("Webhook: %s\n", boolStr(cfg.WebhookURL != "", "configured", "none"))
	fmt.Printf("Queries: %s\n", strings.Join(cfg.SearchQueries, ", "))

	whales, _ := storage.LoadWhales()
	savedMarkets, _ := storage.LoadMarkets()
	fmt.Printf("Tracking %d whales\n", len(whales))
	fmt.Printf("Saved markets: %d\n", len(savedMarkets))
	fmt.Println()

	apiClient := api.New(cfg)
	notify := notifier.New(cfg)
	detect := detector.New(cfg, apiClient, notify.Notify)
	detect.SetWhales(whales)

	wsClient := ws.New(cfg, detect.ProcessWsTrade)

	refresh := func() {
		markets := discoverMarkets(cfg, apiClient, savedMarkets)
		assetIDs := detect.AddMarkets(markets)
		wsClient.Subscribe(assetIDs)
	}

	refresh()
	scanHistoricalTrades(apiClient, detect)

	if err := wsClient.Connect(); err != nil {
		log.Fatalf("WebSocket connection failed: %v", err)
	}

	ticker := time.NewTicker(time.Duration(cfg.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			refresh()
		case <-sigCh:
			fmt.Println("\nShutting down...")
			wsClient.Close()
			return
		}
	}
}

func discoverMarkets(cfg *config.Config, apiClient *api.Client, savedMarkets []types.SavedMarket) []types.Market {
	markets := make(map[string]types.Market)

	if len(savedMarkets) > 0 {
		fmt.Printf("[Discovery] Loading %d saved markets...\n", len(savedMarkets))
		for _, sm := range savedMarkets {
			event, err := apiClient.GetEventBySlug(sm.Slug)
			if err != nil {
				continue
			}
			for _, m := range event.Markets {
				if m.ConditionID != "" && m.ClobTokens != "" {
					markets[m.ConditionID] = m
				}
			}
		}
	}

	for _, query := range cfg.SearchQueries {
		fmt.Printf("[Discovery] Searching: %s\n", query)
		results, err := apiClient.SearchMarkets(query)
		if err != nil {
			fmt.Printf("[Discovery] Search failed: %s\n", query)
			continue
		}
		for _, m := range results {
			if m.ConditionID != "" && m.ClobTokens != "" {
				markets[m.ConditionID] = m
			}
		}
	}

	fmt.Printf("[Discovery] Found %d unique markets\n", len(markets))

	result := make([]types.Market, 0, len(markets))
	for _, m := range markets {
		result = append(result, m)
	}
	return result
}

func scanHistoricalTrades(apiClient *api.Client, detect *detector.Detector) {
	fmt.Println("[Historical] Fetching recent trades...")

	trades, err := apiClient.GetRecentTrades(1000)
	if err != nil {
		fmt.Printf("[Historical] Scan failed: %v\n", err)
		return
	}

	watchedIDs := detect.GetWatchedConditionIDs()
	var relevant []types.Trade
	for _, t := range trades {
		if watchedIDs[t.ConditionID] {
			relevant = append(relevant, t)
		}
	}

	fmt.Printf("[Historical] Found %d/%d trades in watched markets\n", len(relevant), len(trades))

	detected := 0
	for _, t := range relevant {
		if detect.ProcessHistoricalTrade(t) {
			detected++
		}
	}

	fmt.Printf("[Historical] Detected %d fat trades\n", detected)
}

// ============= ADD-MARKET COMMAND =============

func cmdAddMarket(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: polymarket-tool add-market <url-or-slug>")
		os.Exit(1)
	}

	input := args[0]
	slug := parseMarketURL(input)
	if slug == "" {
		slug = input
	}

	cfg := config.Load()
	apiClient := api.New(cfg)

	fmt.Printf("Fetching event: %s...\n", slug)

	event, err := apiClient.GetEventBySlug(slug)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound: %s\n", event.Title)
	fmt.Printf("Markets: %d\n", len(event.Markets))
	fmt.Printf("Volume: $%.2f\n", event.Volume)
	fmt.Printf("Liquidity: $%.2f\n", event.Liquidity)

	if len(event.Markets) > 0 {
		fmt.Println("\nMarket outcomes:")
		for i, m := range event.Markets {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(event.Markets)-5)
				break
			}
			fmt.Printf("  - %s\n", m.Question)
		}
	}

	added, err := storage.AddMarket(types.SavedMarket{
		Slug:    event.Slug,
		Title:   event.Title,
		AddedAt: time.Now().Format(time.RFC3339),
	})

	if err != nil {
		fmt.Printf("Error saving: %v\n", err)
		os.Exit(1)
	}

	if added {
		fmt.Println("\n✓ Market added to watch list")
	} else {
		fmt.Println("\nMarket already in watch list")
	}

	markets, _ := storage.LoadMarkets()
	fmt.Printf("Current watched markets: %d\n", len(markets))
}

func parseMarketURL(input string) string {
	re := regexp.MustCompile(`polymarket\.com/event/([a-z0-9-]+)`)
	matches := re.FindStringSubmatch(input)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// ============= DISCOVER-WHALES COMMAND =============

func cmdDiscoverWhales(args []string) {
	cfg := config.Load()
	apiClient := api.New(cfg)

	fmt.Println("Discover Whales from Leaderboard")
	fmt.Println("=================================\n")

	existingWhales, _ := storage.LoadWhales()
	existingAddrs := make(map[string]bool)
	for _, w := range existingWhales {
		existingAddrs[strings.ToLower(w.Address)] = true
	}

	fmt.Printf("Currently tracking %d whales\n\n", len(existingWhales))
	fmt.Println("Fetching leaderboard...\n")

	leaderboard, err := apiClient.GetLeaderboard(30)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Top traders by PnL:\n")
	fmt.Println("  #  | Name                 | PnL          | Volume       | Status")
	fmt.Println("-----+----------------------+--------------+--------------+--------")

	for _, entry := range leaderboard {
		status := ""
		if existingAddrs[strings.ToLower(entry.ProxyWallet)] {
			status = "✓ tracked"
		}
		name := entry.UserName
		if name == "" {
			name = entry.ProxyWallet[:12]
		}
		if len(name) > 20 {
			name = name[:20]
		}
		fmt.Printf("  %2s | %-20s | %12s | %12s | %s\n",
			entry.Rank, name, formatUSD(entry.PnL), formatUSD(entry.Volume), status)
	}

	fmt.Println()

	var selection string
	if len(args) > 0 {
		selection = args[0]
	} else {
		fmt.Print("Enter ranks to add (comma-separated), 'all', or 'topN': ")
		reader := bufio.NewReader(os.Stdin)
		selection, _ = reader.ReadString('\n')
		selection = strings.TrimSpace(selection)
	}

	if selection == "" {
		return
	}

	var toAdd []types.LeaderboardEntry

	switch {
	case strings.ToLower(selection) == "all":
		for _, e := range leaderboard {
			if !existingAddrs[strings.ToLower(e.ProxyWallet)] {
				toAdd = append(toAdd, e)
			}
		}
	case strings.HasPrefix(strings.ToLower(selection), "top"):
		n, _ := strconv.Atoi(selection[3:])
		if n == 0 {
			n = 10
		}
		for i, e := range leaderboard {
			if i >= n {
				break
			}
			if !existingAddrs[strings.ToLower(e.ProxyWallet)] {
				toAdd = append(toAdd, e)
			}
		}
	default:
		ranks := strings.Split(selection, ",")
		rankSet := make(map[string]bool)
		for _, r := range ranks {
			rankSet[strings.TrimSpace(r)] = true
		}
		for _, e := range leaderboard {
			if rankSet[e.Rank] && !existingAddrs[strings.ToLower(e.ProxyWallet)] {
				toAdd = append(toAdd, e)
			}
		}
	}

	if len(toAdd) == 0 {
		fmt.Println("No new whales to add.")
		return
	}

	fmt.Printf("\nAdding %d whales...\n", len(toAdd))

	for _, entry := range toAdd {
		name := entry.UserName
		if name == "" {
			name = "Rank #" + entry.Rank
		}

		whale := types.Whale{
			Address: entry.ProxyWallet,
			Name:    name,
			PnL:     entry.PnL,
			Volume:  entry.Volume,
			AddedAt: time.Now().Format(time.RFC3339),
		}

		if added, _ := storage.AddWhale(whale); added {
			fmt.Printf("  ✓ Added: %s (%s PnL)\n", name, formatUSD(entry.PnL))
		}
	}

	whales, _ := storage.LoadWhales()
	fmt.Printf("\nNow tracking %d whales total.\n", len(whales))
}

// ============= WHALE-TRADES COMMAND =============

func cmdWhaleTrades(args []string) {
	whales, _ := storage.LoadWhales()

	if len(whales) == 0 {
		fmt.Println("No whales tracked. Run: polymarket-tool discover-whales")
		return
	}

	selectedWhales := whales
	limit := 20

	if len(args) > 0 {
		selection := args[0]
		idx, err := strconv.Atoi(selection)
		if err == nil && idx > 0 && idx <= len(whales) {
			selectedWhales = []types.Whale{whales[idx-1]}
		} else {
			var filtered []types.Whale
			for _, w := range whales {
				if strings.Contains(strings.ToLower(w.Name), strings.ToLower(selection)) ||
					strings.Contains(strings.ToLower(w.Address), strings.ToLower(selection)) {
					filtered = append(filtered, w)
				}
			}
			if len(filtered) > 0 {
				selectedWhales = filtered
			} else {
				fmt.Printf("No whale found matching: %s\n", selection)
				fmt.Println("\nTracked whales:")
				for i, w := range whales {
					fmt.Printf("  %d. %s\n", i+1, w.Name)
				}
				return
			}
		}
	}

	if len(args) > 1 {
		if l, err := strconv.Atoi(args[1]); err == nil {
			limit = l
		}
	}

	cfg := config.Load()
	apiClient := api.New(cfg)

	for _, whale := range selectedWhales {
		fmt.Printf("\n%s\n", strings.Repeat("=", 70))
		fmt.Printf("🐋 %s\n", whale.Name)
		fmt.Printf("   %s\n", whale.Address)
		fmt.Printf("   PnL: %s | Volume: %s\n", formatUSD(whale.PnL), formatUSD(whale.Volume))
		fmt.Println(strings.Repeat("=", 70))

		activity, err := apiClient.GetUserActivity(whale.Address, limit)
		if err != nil {
			fmt.Printf("\n  Error fetching activity: %v\n", err)
			continue
		}

		var trades []types.UserActivity
		for _, a := range activity {
			if a.Type == "TRADE" {
				trades = append(trades, a)
			}
		}

		if len(trades) == 0 {
			fmt.Println("\n  No recent trades found.\n")
			continue
		}

		fmt.Printf("\nRecent trades (%d):\n\n", len(trades))
		fmt.Println("  Time       | Side | Value        | Market")
		fmt.Println("  " + strings.Repeat("-", 66))

		var totalValue float64
		marketSet := make(map[string]bool)

		for _, t := range trades {
			timeStr := formatTimeAgo(t.Timestamp)
			side := t.Side
			if side == "" {
				side = "TRADE"
			}
			title := t.Title
			if len(title) > 40 {
				title = title[:40]
			}
			fmt.Printf("  %-10s | %-4s | %12s | %s\n",
				timeStr, side, formatUSD(t.UsdcSize), title)
			totalValue += t.UsdcSize
			marketSet[t.ConditionID] = true
		}

		fmt.Println("\n  " + strings.Repeat("-", 66))
		fmt.Printf("  Total: %s across %d markets\n", formatUSD(totalValue), len(marketSet))
	}

	fmt.Println()
}

// ============= LIST COMMAND =============

func cmdList(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: polymarket-tool list <whales|markets|remove-whale|remove-market>")
		return
	}

	switch args[0] {
	case "whales":
		whales, _ := storage.LoadWhales()
		fmt.Printf("\nTracked Whales (%d):\n", len(whales))
		fmt.Println(strings.Repeat("=", 70))

		if len(whales) == 0 {
			fmt.Println("No whales tracked. Run: polymarket-tool discover-whales")
		} else {
			for _, w := range whales {
				fmt.Printf("  %s\n", w.Name)
				fmt.Printf("    Address: %s\n", w.Address)
				fmt.Printf("    PnL: %s | Volume: %s\n", formatUSD(w.PnL), formatUSD(w.Volume))
				if w.Note != "" {
					fmt.Printf("    Note: %s\n", w.Note)
				}
				fmt.Println()
			}
		}

	case "markets":
		markets, _ := storage.LoadMarkets()
		fmt.Printf("\nSaved Markets (%d):\n", len(markets))
		fmt.Println(strings.Repeat("=", 70))

		if len(markets) == 0 {
			fmt.Println("No markets saved. Run: polymarket-tool add-market <url>")
		} else {
			for _, m := range markets {
				fmt.Printf("  %s\n", m.Title)
				fmt.Printf("    Slug: %s\n", m.Slug)
				fmt.Printf("    URL: https://polymarket.com/event/%s\n", m.Slug)
				fmt.Println()
			}
		}

	case "remove-whale":
		if len(args) < 2 {
			fmt.Println("Usage: polymarket-tool list remove-whale <address>")
			return
		}
		if removed, _ := storage.RemoveWhale(args[1]); removed {
			fmt.Println("✓ Whale removed")
		} else {
			fmt.Println("Whale not found")
		}

	case "remove-market":
		if len(args) < 2 {
			fmt.Println("Usage: polymarket-tool list remove-market <slug>")
			return
		}
		if removed, _ := storage.RemoveMarket(args[1]); removed {
			fmt.Println("✓ Market removed")
		} else {
			fmt.Println("Market not found")
		}

	default:
		fmt.Printf("Unknown list type: %s\n", args[0])
	}
}

// ============= HELPERS =============

func formatUSD(value float64) string {
	if value >= 1_000_000 {
		return fmt.Sprintf("$%.2fM", value/1_000_000)
	}
	if value >= 1_000 {
		return fmt.Sprintf("$%.1fK", value/1_000)
	}
	return fmt.Sprintf("$%.2f", value)
}

func formatTimeAgo(ts int64) string {
	t := time.Unix(ts, 0)
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}

func boolStr(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}
