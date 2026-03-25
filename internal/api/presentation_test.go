package api

import (
	"testing"
	"time"

	"github.com/numofx/matching-backend/internal/instruments"
	"github.com/numofx/matching-backend/internal/orders"
)

func TestPresentOrderForBTCVar30AddsVarianceAndVolDisplay(t *testing.T) {
	order := orders.Order{
		LimitPrice:      "0.2728",
		LimitPriceTicks: "2728",
	}
	instrument := instruments.Metadata{
		Symbol:           "BTCVAR30-PERP",
		TickSize:         "0.0001",
		QuotePrecision:   6,
		PricingModel:     instruments.PricingModelVariance,
		PriceSemantics:   instruments.PricingModelVariance,
		DisplayPriceKind: instruments.DisplayPriceVolPercent,
	}

	presented := presentOrder(order, instrument)
	if presented.VariancePrice != 0.2728 {
		t.Fatalf("variance price = %v", presented.VariancePrice)
	}
	if presented.VolPercent != 52.23 {
		t.Fatalf("vol percent = %v", presented.VolPercent)
	}
	if presented.PriceSemantics != instruments.PricingModelVariance {
		t.Fatalf("price semantics = %s", presented.PriceSemantics)
	}
}

func TestPresentOrderForLinearInstrumentLeavesVarianceFieldsEmpty(t *testing.T) {
	order := orders.Order{
		LimitPrice:      "100",
		LimitPriceTicks: "100",
	}
	instrument := instruments.Metadata{
		Symbol:           "BTCUSDC-CVXPERP",
		TickSize:         "1",
		QuotePrecision:   8,
		PricingModel:     instruments.PricingModelLinear,
		PriceSemantics:   instruments.PricingModelLinear,
		DisplayPriceKind: instruments.DisplayPriceDirect,
	}

	presented := presentOrder(order, instrument)
	if presented.VariancePrice != 0 || presented.VolPercent != 0 {
		t.Fatalf("unexpected variance presentation %+v", presented)
	}
}

func TestPresentTradesIncludesDeliverableMetadata(t *testing.T) {
	items := []orders.TradeFill{{
		TradeID:       1,
		AssetAddress:  "0xf000000000000000000000000000000000000123",
		SubID:         "1777507200",
		Price:         "1605.25",
		Size:          "100000000000000000",
		AggressorSide: orders.SideBuy,
		TakerOrderID:  "taker-1",
		MakerOrderID:  "maker-1",
		CreatedAt:     time.Unix(1777507200, 0).UTC(),
	}}
	instrument := instruments.Metadata{
		Symbol:         instruments.CNGNApr2026Symbol,
		ContractType:   "deliverable_fx_future",
		SettlementType: "physical_delivery",
	}

	presented := presentTrades(items, instrument)
	if len(presented) != 1 {
		t.Fatalf("len = %d", len(presented))
	}
	if presented[0].Market != instruments.CNGNApr2026Symbol {
		t.Fatalf("market = %q", presented[0].Market)
	}
	if presented[0].ContractType != "deliverable_fx_future" {
		t.Fatalf("contract type = %q", presented[0].ContractType)
	}
	if presented[0].SettlementType != "physical_delivery" {
		t.Fatalf("settlement type = %q", presented[0].SettlementType)
	}
}

func TestPresentOrderForSpotIncludesNormalizedUIAndEngineEcho(t *testing.T) {
	order := orders.Order{
		Side:          orders.SideSell,
		DesiredAmount: "160525",
		LimitPrice:    "0.000622955925867933",
	}
	instrument := instruments.Metadata{
		Symbol:         instruments.CNGNSpotSymbol,
		ContractType:   "spot",
		SettlementType: "spot",
	}

	presented := presentOrder(order, instrument)
	if presented.SpotContract == nil {
		t.Fatal("expected spot contract echo")
	}
	if presented.SpotContract.UIIntent.Side != "buy" {
		t.Fatalf("ui side = %q", presented.SpotContract.UIIntent.Side)
	}
	if presented.SpotContract.UIIntent.Price != "1605.25" {
		t.Fatalf("ui price = %q", presented.SpotContract.UIIntent.Price)
	}
	if presented.SpotContract.UIIntent.Size != "100" {
		t.Fatalf("ui size = %q", presented.SpotContract.UIIntent.Size)
	}
	if presented.SpotContract.BalanceDelta.USDC != "+100" || presented.SpotContract.BalanceDelta.CNGN != "-160525" {
		t.Fatalf("unexpected balance deltas %+v", presented.SpotContract.BalanceDelta)
	}
}

func TestPresentTradeForSpotIncludesNormalizedUIAndEngineEcho(t *testing.T) {
	items := []orders.TradeFill{{
		TradeID:       1,
		AssetAddress:  "0xf000000000000000000000000000000000000999",
		SubID:         "0",
		Price:         "0.000622955925867933",
		Size:          "160525",
		AggressorSide: orders.SideSell,
		CreatedAt:     time.Unix(1777507200, 0).UTC(),
	}}
	instrument := instruments.Metadata{
		Symbol:         instruments.CNGNSpotSymbol,
		ContractType:   "spot",
		SettlementType: "spot",
	}

	presented := presentTrades(items, instrument)
	if len(presented) != 1 || presented[0].SpotContract == nil {
		t.Fatalf("unexpected presented trades %+v", presented)
	}
	if presented[0].SpotContract.EngineOrder.Side != "sell" {
		t.Fatalf("engine side = %q", presented[0].SpotContract.EngineOrder.Side)
	}
	if presented[0].SpotContract.UIIntent.Side != "buy" {
		t.Fatalf("ui side = %q", presented[0].SpotContract.UIIntent.Side)
	}
}

func TestPresentMarketIncludesSpotOrderEntryContract(t *testing.T) {
	instrument := instruments.Metadata{
		Symbol:           instruments.CNGNSpotSymbol,
		ContractType:     "spot",
		SettlementType:   "spot",
		OrderEntrySpec:   "usdc_cngn_spot_v1",
		UIPriceUnit:      "cNGN per USDC",
		UISizeUnit:       "USDC notional",
		UISideMeaning:    "BUY acquires USDC and sells cNGN inventory; SELL delivers USDC and buys cNGN inventory.",
		EnginePriceUnit:  "USDC per cNGN",
		EngineAmountUnit: "cNGN amount",
		EngineSidePolicy: "invert_ui_side",
		UIPriceToEngine:  "engine_price = 1 / ui_price",
		UISizeToEngine:   "engine_amount = ui_size * ui_price",
	}

	presented := presentMarket(instrument)
	if presented.OrderEntrySpec != "usdc_cngn_spot_v1" {
		t.Fatalf("order entry spec = %q", presented.OrderEntrySpec)
	}
	if presented.EngineSidePolicy != "invert_ui_side" {
		t.Fatalf("engine side policy = %q", presented.EngineSidePolicy)
	}
	if presented.UIPriceToEngine != "engine_price = 1 / ui_price" {
		t.Fatalf("ui price formula = %q", presented.UIPriceToEngine)
	}
	if presented.UISizeToEngine != "engine_amount = ui_size * ui_price" {
		t.Fatalf("ui size formula = %q", presented.UISizeToEngine)
	}
}
