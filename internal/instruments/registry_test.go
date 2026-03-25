package instruments

import (
	"testing"

	"github.com/numofx/matching-backend/internal/config"
)

func TestDefaultRegistryIncludesDeliverableFutureByAssetAndSubID(t *testing.T) {
	cfg := config.Config{
		CNGNApr2026FutureAssetAddress: "0xF000000000000000000000000000000000000123",
		CNGNApr2026FutureSubID:        "1777507200",
	}

	registry := DefaultRegistry(cfg)

	item, ok := registry.ByAssetAndSubID("0xf000000000000000000000000000000000000123", "1777507200")
	if !ok {
		t.Fatalf("deliverable future not found by asset/subId")
	}

	if item.Symbol != CNGNApr2026Symbol {
		t.Fatalf("symbol = %q", item.Symbol)
	}
	if item.ContractType != "deliverable_fx_future" {
		t.Fatalf("contract type = %q", item.ContractType)
	}
	if item.SettlementType != "physical_delivery" {
		t.Fatalf("settlement type = %q", item.SettlementType)
	}
	if item.BaseAssetSymbol != "USDC" || item.QuoteAssetSymbol != "cNGN" {
		t.Fatalf("unexpected base/quote %q/%q", item.BaseAssetSymbol, item.QuoteAssetSymbol)
	}
	if item.SubID != "1777507200" {
		t.Fatalf("subId = %q", item.SubID)
	}
	if !item.Enabled {
		t.Fatalf("deliverable future should be enabled when both env vars are set")
	}
}

func TestDefaultRegistryIncludesSpotByAssetAndSubID(t *testing.T) {
	cfg := config.Config{
		CNGNSpotAssetAddress: "0xF000000000000000000000000000000000000999",
	}

	registry := DefaultRegistry(cfg)

	item, ok := registry.ByAssetAndSubID("0xf000000000000000000000000000000000000999", "0")
	if !ok {
		t.Fatalf("spot market not found by asset/subId")
	}

	if item.Symbol != CNGNSpotSymbol {
		t.Fatalf("symbol = %q", item.Symbol)
	}
	if item.ContractType != "spot" {
		t.Fatalf("contract type = %q", item.ContractType)
	}
	if item.SettlementType != "spot" {
		t.Fatalf("settlement type = %q", item.SettlementType)
	}
	if item.BaseAssetSymbol != "USDC" || item.QuoteAssetSymbol != "cNGN" {
		t.Fatalf("unexpected base/quote %q/%q", item.BaseAssetSymbol, item.QuoteAssetSymbol)
	}
	if item.SubID != "0" {
		t.Fatalf("subId = %q", item.SubID)
	}
	if !item.Enabled {
		t.Fatalf("spot market should be enabled when asset address is set")
	}
}

func TestDefaultRegistryResolvesLegacyDisplaySymbolAlias(t *testing.T) {
	cfg := config.Config{
		CNGNApr2026FutureAssetAddress: "0xF000000000000000000000000000000000000123",
		CNGNApr2026FutureSubID:        "1777507200",
	}

	registry := DefaultRegistry(cfg)

	item, ok := registry.BySymbol(CNGNApr2026LegacySymbol)
	if !ok {
		t.Fatalf("deliverable future not found by legacy display symbol")
	}
	if item.Symbol != CNGNApr2026Symbol {
		t.Fatalf("canonical symbol = %q", item.Symbol)
	}
}

func TestDefaultRegistryResolvesSpotLegacyDisplaySymbolAlias(t *testing.T) {
	cfg := config.Config{
		CNGNSpotAssetAddress: "0xF000000000000000000000000000000000000999",
	}

	registry := DefaultRegistry(cfg)

	item, ok := registry.BySymbol(CNGNSpotLegacySymbol)
	if !ok {
		t.Fatalf("spot market not found by legacy display symbol")
	}
	if item.Symbol != CNGNSpotSymbol {
		t.Fatalf("canonical symbol = %q", item.Symbol)
	}
}
