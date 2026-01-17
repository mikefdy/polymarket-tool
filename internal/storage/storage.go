package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mikefdy/polymarket-tool/internal/types"
)

const dataDir = "data"

func ensureDataDir() error {
	return os.MkdirAll(dataDir, 0755)
}

func LoadWhales() ([]types.Whale, error) {
	if err := ensureDataDir(); err != nil {
		return nil, err
	}

	path := filepath.Join(dataDir, "whales.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []types.Whale{}, nil
		}
		return nil, err
	}

	var whales []types.Whale
	if err := json.Unmarshal(data, &whales); err != nil {
		return nil, err
	}
	return whales, nil
}

func SaveWhales(whales []types.Whale) error {
	if err := ensureDataDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(whales, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dataDir, "whales.json"), data, 0644)
}

func AddWhale(whale types.Whale) (bool, error) {
	whales, err := LoadWhales()
	if err != nil {
		return false, err
	}

	for _, w := range whales {
		if strings.EqualFold(w.Address, whale.Address) {
			return false, nil
		}
	}

	whales = append(whales, whale)
	return true, SaveWhales(whales)
}

func RemoveWhale(address string) (bool, error) {
	whales, err := LoadWhales()
	if err != nil {
		return false, err
	}

	filtered := make([]types.Whale, 0, len(whales))
	found := false
	for _, w := range whales {
		if strings.EqualFold(w.Address, address) {
			found = true
		} else {
			filtered = append(filtered, w)
		}
	}

	if !found {
		return false, nil
	}

	return true, SaveWhales(filtered)
}

func LoadMarkets() ([]types.SavedMarket, error) {
	if err := ensureDataDir(); err != nil {
		return nil, err
	}

	path := filepath.Join(dataDir, "markets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []types.SavedMarket{}, nil
		}
		return nil, err
	}

	var markets []types.SavedMarket
	if err := json.Unmarshal(data, &markets); err != nil {
		return nil, err
	}
	return markets, nil
}

func SaveMarkets(markets []types.SavedMarket) error {
	if err := ensureDataDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(markets, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dataDir, "markets.json"), data, 0644)
}

func AddMarket(market types.SavedMarket) (bool, error) {
	markets, err := LoadMarkets()
	if err != nil {
		return false, err
	}

	for _, m := range markets {
		if m.Slug == market.Slug {
			return false, nil
		}
	}

	markets = append(markets, market)
	return true, SaveMarkets(markets)
}

func RemoveMarket(slug string) (bool, error) {
	markets, err := LoadMarkets()
	if err != nil {
		return false, err
	}

	filtered := make([]types.SavedMarket, 0, len(markets))
	found := false
	for _, m := range markets {
		if m.Slug == slug {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}

	if !found {
		return false, nil
	}

	return true, SaveMarkets(filtered)
}

func ClearMarkets() (int, error) {
	markets, err := LoadMarkets()
	if err != nil {
		return 0, err
	}
	count := len(markets)
	return count, SaveMarkets([]types.SavedMarket{})
}

func ClearWhales() (int, error) {
	whales, err := LoadWhales()
	if err != nil {
		return 0, err
	}
	count := len(whales)
	return count, SaveWhales([]types.Whale{})
}
