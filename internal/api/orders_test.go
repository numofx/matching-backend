package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/numofx/matching-backend/internal/config"
	"github.com/numofx/matching-backend/internal/orders"
)

func TestCreateOrderRequestToParamsRejectsActionJSONOwnerMismatch(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-1",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xdef",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xasset",
		SubID:         "0",
		DesiredAmount: "100",
		FilledAmount:  "0",
		LimitPrice:    "75",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xwrong","signer":"0xdef"}`),
		Signature:     "0xsig",
	}

	_, err := req.toParams(config.Config{})
	if err == nil || err.Error() != "action_json.owner must match owner_address" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateOrderRequestToParamsRejectsUnexpectedConfiguredSigner(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-1",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xdef",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xasset",
		SubID:         "0",
		DesiredAmount: "100",
		FilledAmount:  "0",
		LimitPrice:    "75",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xdef"}`),
		Signature:     "0xsig",
	}

	_, err := req.toParams(config.Config{ExpectedOrderSigner: "0x123"})
	if err == nil || err.Error() != "signer_address must match configured expected signer" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateOrderRequestToParamsParsesBTCVar30DecimalPrice(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-1",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xvar",
		SubID:         "0",
		DesiredAmount: "100",
		FilledAmount:  "0",
		LimitPrice:    "0.2724",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	params, err := req.toParams(config.Config{
		BTCVar30Enabled:      true,
		BTCVar30AssetAddress: "0xvar",
	})
	if err != nil {
		t.Fatalf("toParams returned error: %v", err)
	}
	if params.LimitPrice != "0.2724" {
		t.Fatalf("display limit price = %s", params.LimitPrice)
	}
	if params.LimitPriceTicks != "2724" {
		t.Fatalf("limit price ticks = %s", params.LimitPriceTicks)
	}
}

func TestCreateOrderRequestToParamsRejectsOffTickBTCVar30Price(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-1",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xvar",
		SubID:         "0",
		DesiredAmount: "100",
		FilledAmount:  "0",
		LimitPrice:    "0.27245",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	_, err := req.toParams(config.Config{
		BTCVar30Enabled:      true,
		BTCVar30AssetAddress: "0xvar",
	})
	if err == nil || err.Error() != "price must align to tick size 0.0001" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateOrderRequestToParamsRejectsVolLookingBTCVar30Price(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-1",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xvar",
		SubID:         "0",
		DesiredAmount: "100",
		FilledAmount:  "0",
		LimitPrice:    "52.0",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	_, err := req.toParams(config.Config{
		BTCVar30Enabled:      true,
		BTCVar30AssetAddress: "0xvar",
	})
	if err == nil || err.Error() != "BTCVAR30 prices are variance, not volatility. Example: 0.25 = 50% vol" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateOrderRequestToParamsAcceptsVarianceLookingBTCVar30Price(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-1",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xvar",
		SubID:         "0",
		DesiredAmount: "100",
		FilledAmount:  "0",
		LimitPrice:    "0.52",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	params, err := req.toParams(config.Config{
		BTCVar30Enabled:      true,
		BTCVar30AssetAddress: "0xvar",
	})
	if err != nil {
		t.Fatalf("toParams returned error: %v", err)
	}
	if params.LimitPrice != "0.52" || params.LimitPriceTicks != "5200" {
		t.Fatalf("unexpected params %+v", params)
	}
}

func TestCreateOrderRequestToParamsAcceptsSpotInstrument(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-spot-1",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "sell",
		AssetAddress:  "0xspot",
		SubID:         "0",
		DesiredAmount: "160525",
		FilledAmount:  "0",
		LimitPrice:    "0.000622955925867933",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	params, err := req.toParams(config.Config{
		CNGNSpotAssetAddress: "0xspot",
	})
	if err != nil {
		t.Fatalf("toParams returned error: %v", err)
	}
	if params.AssetAddress != "0xspot" || params.SubID != "0" {
		t.Fatalf("unexpected asset/subId %+v", params)
	}
	if params.LimitPrice != "0.000622955925867933" || params.LimitPriceTicks != "622955925867933" {
		t.Fatalf("unexpected price fields %+v", params)
	}
}

func TestCreateOrderRequestToParamsValidatesSpotUIIntentContract(t *testing.T) {
	req := createOrderRequest{
		OrderID:        "order-spot-ui-1",
		OwnerAddress:   "0xabc",
		SignerAddress:  "0xabc",
		SubaccountID:   "10",
		RecipientID:    "10",
		Nonce:          "1",
		AssetAddress:   "0xspot",
		SubID:          "0",
		FilledAmount:   "0",
		WorstFee:       "1",
		Expiry:         time.Now().Add(time.Hour).Unix(),
		OrderEntrySpec: "usdc_cngn_spot_v1",
		UIIntent: &spotOrderIntent{
			Side:  "buy",
			Price: "1605.25",
			Size:  "100",
		},
		ActionJSON: json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:  "0xsig",
	}

	params, err := req.toParams(config.Config{
		CNGNSpotAssetAddress: "0xspot",
	})
	if err != nil {
		t.Fatalf("toParams returned error: %v", err)
	}
	if params.Side != orders.SideSell {
		t.Fatalf("side = %s", params.Side)
	}
	if params.LimitPrice != "0.000622955925868245" {
		t.Fatalf("limit price = %s", params.LimitPrice)
	}
	if params.DesiredAmount != "160525" {
		t.Fatalf("desired amount = %s", params.DesiredAmount)
	}
}

func TestCreateOrderRequestToParamsRejectsMalformedSpotUIIntentContract(t *testing.T) {
	req := createOrderRequest{
		OrderID:        "order-spot-ui-1",
		OwnerAddress:   "0xabc",
		SignerAddress:  "0xabc",
		SubaccountID:   "10",
		RecipientID:    "10",
		Nonce:          "1",
		AssetAddress:   "0xspot",
		SubID:          "0",
		Side:           "buy",
		DesiredAmount:  "160525",
		LimitPrice:     "0.000622955925868245",
		FilledAmount:   "0",
		WorstFee:       "1",
		Expiry:         time.Now().Add(time.Hour).Unix(),
		OrderEntrySpec: "usdc_cngn_spot_v1",
		UIIntent: &spotOrderIntent{
			Side:  "buy",
			Price: "1605.26",
			Size:  "100",
		},
		ActionJSON: json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:  "0xsig",
	}

	_, err := req.toParams(config.Config{
		CNGNSpotAssetAddress: "0xspot",
	})
	if err == nil || err.Error() != "side does not match ui_intent translation" {
		t.Fatalf("unexpected error: %v", err)
	}
}
