package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/numofx/matching-backend/internal/config"
	"github.com/numofx/matching-backend/internal/instruments"
)

func TestHandleMarketsIncludesDeliverableFutureMetadata(t *testing.T) {
	registry := instruments.DefaultRegistry(config.Config{
		BTCPerpAssetAddress:           "0xbtc",
		CNGNSpotAssetAddress:          "0xf000000000000000000000000000000000000999",
		CNGNApr2026FutureAssetAddress: "0xf000000000000000000000000000000000000123",
		CNGNApr2026FutureSubID:        "1777507200",
	})

	server := NewServer(config.Config{}, nil, registry, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/markets", nil)
	rec := httptest.NewRecorder()
	server.handleMarkets(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var markets []marketPresentation
	if err := json.Unmarshal(rec.Body.Bytes(), &markets); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	var found *marketPresentation
	for i := range markets {
		if markets[i].Market == instruments.CNGNApr2026Symbol {
			found = &markets[i]
			break
		}
	}
	if found == nil {
		t.Fatal("deliverable future missing from markets response")
	}

	if found.ContractType != "deliverable_fx_future" {
		t.Fatalf("contract type = %q", found.ContractType)
	}
	if found.SettlementType != "physical_delivery" {
		t.Fatalf("settlement type = %q", found.SettlementType)
	}
	if found.AssetAddress != "0xf000000000000000000000000000000000000123" {
		t.Fatalf("asset address = %q", found.AssetAddress)
	}
	if found.SubID != "1777507200" {
		t.Fatalf("sub id = %q", found.SubID)
	}
	if found.ExpiryTimestamp != 1777507200 {
		t.Fatalf("unexpected expiry window %+v", found)
	}
	if found.LastTradeTimestamp != nil {
		t.Fatalf("expected nil last_trade_timestamp without trade history, got %+v", found)
	}
	if found.BaseAssetSymbol != "USDC" || found.QuoteAssetSymbol != "cNGN" {
		t.Fatalf("unexpected base/quote %q/%q", found.BaseAssetSymbol, found.QuoteAssetSymbol)
	}
	if found.TickSize != "1" {
		t.Fatalf("tick size = %q", found.TickSize)
	}
}

func TestHandleMarketsIncludesSpotAndFutureMetadata(t *testing.T) {
	registry := instruments.DefaultRegistry(config.Config{
		CNGNSpotAssetAddress:          "0xf000000000000000000000000000000000000999",
		CNGNApr2026FutureAssetAddress: "0xf000000000000000000000000000000000000123",
		CNGNApr2026FutureSubID:        "1777507200",
	})

	server := NewServer(config.Config{}, nil, registry, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/markets", nil)
	rec := httptest.NewRecorder()
	server.handleMarkets(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var markets []marketPresentation
	if err := json.Unmarshal(rec.Body.Bytes(), &markets); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	var foundSpot *marketPresentation
	var foundFuture *marketPresentation
	for i := range markets {
		switch markets[i].Market {
		case instruments.CNGNSpotSymbol:
			foundSpot = &markets[i]
		case instruments.CNGNApr2026Symbol:
			foundFuture = &markets[i]
		}
	}

	if foundSpot == nil {
		t.Fatal("spot market missing from markets response")
	}
	if foundSpot.ContractType != "spot" || foundSpot.SettlementType != "spot" {
		t.Fatalf("unexpected spot metadata %+v", *foundSpot)
	}
	if foundSpot.AssetAddress != "0xf000000000000000000000000000000000000999" || foundSpot.SubID != "0" {
		t.Fatalf("unexpected spot asset/subId %+v", *foundSpot)
	}
	if foundSpot.BaseAssetSymbol != "USDC" || foundSpot.QuoteAssetSymbol != "cNGN" {
		t.Fatalf("unexpected spot base/quote %q/%q", foundSpot.BaseAssetSymbol, foundSpot.QuoteAssetSymbol)
	}
	if foundSpot.TickSize != "0.000000000000000001" {
		t.Fatalf("unexpected spot tick size %q", foundSpot.TickSize)
	}
	if foundSpot.OrderEntrySpec != "usdc_cngn_spot_v1" {
		t.Fatalf("unexpected spot order entry spec %q", foundSpot.OrderEntrySpec)
	}
	if foundSpot.EngineSidePolicy != "invert_ui_side" {
		t.Fatalf("unexpected spot engine side policy %q", foundSpot.EngineSidePolicy)
	}
	if foundSpot.UIPriceToEngine != "engine_price = 1 / ui_price" {
		t.Fatalf("unexpected spot price formula %q", foundSpot.UIPriceToEngine)
	}
	if foundSpot.UISizeToEngine != "engine_amount = ui_size * ui_price" {
		t.Fatalf("unexpected spot size formula %q", foundSpot.UISizeToEngine)
	}

	if foundFuture == nil {
		t.Fatal("future market missing from markets response")
	}
}
