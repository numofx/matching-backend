package api

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
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

func TestCreateOrderRequestToParamsNormalizesAPRMinimumSizeToAtomicUnits(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-apr-min-size",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xapr",
		SubID:         "1777507200",
		DesiredAmount: "0.001",
		FilledAmount:  "0",
		LimitPrice:    "1391",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	params, err := req.toParams(config.Config{
		CNGNApr2026FutureAssetAddress: "0xapr",
		CNGNApr2026FutureSubID:        "1777507200",
	})
	if err != nil {
		t.Fatalf("toParams returned error: %v", err)
	}
	if params.DesiredAmount != "1" {
		t.Fatalf("desired amount = %s", params.DesiredAmount)
	}
	if params.FilledAmount != "0" {
		t.Fatalf("filled amount = %s", params.FilledAmount)
	}
}

func TestCreateOrderRequestToParamsRejectsAPRSubMinimumSize(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-apr-sub-min-size",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xapr",
		SubID:         "1777507200",
		DesiredAmount: "0.0001",
		FilledAmount:  "0",
		LimitPrice:    "1391",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	_, err := req.toParams(config.Config{
		CNGNApr2026FutureAssetAddress: "0xapr",
		CNGNApr2026FutureSubID:        "1777507200",
	})
	if err == nil || err.Error() != "desired_amount must align to amount step 0.001" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateOrderRequestToParamsRejectsZeroNormalizedAtomicSize(t *testing.T) {
	req := createOrderRequest{
		OrderID:       "order-apr-zero-size",
		OwnerAddress:  "0xabc",
		SignerAddress: "0xabc",
		SubaccountID:  "10",
		RecipientID:   "10",
		Nonce:         "1",
		Side:          "buy",
		AssetAddress:  "0xapr",
		SubID:         "1777507200",
		DesiredAmount: "0",
		FilledAmount:  "0",
		LimitPrice:    "1391",
		WorstFee:      "1",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xabc","signer":"0xabc"}`),
		Signature:     "0xsig",
	}

	_, err := req.toParams(config.Config{
		CNGNApr2026FutureAssetAddress: "0xapr",
		CNGNApr2026FutureSubID:        "1777507200",
	})
	if err == nil || err.Error() != "normalized atomic size is 0" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateOrderRequestToParamsEnforcesActionDataScaleInvariant(t *testing.T) {
	asset := "0xce2846771074e20fec739cf97a60e6075d1e464b"
	req := createOrderRequest{
		OrderID:       "order-apr-scale-check",
		OwnerAddress:  "0xc7be60b228b997c23094ddfdd71e22e2de6c9310",
		SignerAddress: "0xc7be60b228b997c23094ddfdd71e22e2de6c9310",
		SubaccountID:  "7",
		RecipientID:   "7",
		Nonce:         "11",
		Side:          "buy",
		AssetAddress:  asset,
		SubID:         "1777507200",
		DesiredAmount: "0.001",
		FilledAmount:  "0",
		LimitPrice:    "1391",
		WorstFee:      "0",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON: json.RawMessage(`{
			"subaccount_id":"7",
			"nonce":"11",
			"module":"0x0aae65aaa66fe7f54486cdbd007956d3de611990",
			"data":"` + mustTradeDataHex(asset, "1777507200", "1391000000000000000000", "1000000000000000", true) + `",
			"expiry":"1777507200",
			"owner":"0xc7be60b228b997c23094ddfdd71e22e2de6c9310",
			"signer":"0xc7be60b228b997c23094ddfdd71e22e2de6c9310"
		}`),
		Signature: "0xsig",
	}

	params, err := req.toParams(config.Config{
		CNGNApr2026FutureAssetAddress: asset,
		CNGNApr2026FutureSubID:        "1777507200",
		EnforceActionDataInvariants:   true,
	})
	if err != nil {
		t.Fatalf("toParams returned error: %v", err)
	}
	if params.LimitPriceTicks != "1391" || params.DesiredAmount != "1" {
		t.Fatalf("unexpected normalized params %+v", params)
	}
}

func TestCreateOrderRequestToParamsRejectsActionDataScaleMismatch(t *testing.T) {
	asset := "0xce2846771074e20fec739cf97a60e6075d1e464b"
	req := createOrderRequest{
		OrderID:       "order-apr-scale-mismatch",
		OwnerAddress:  "0xc7be60b228b997c23094ddfdd71e22e2de6c9310",
		SignerAddress: "0xc7be60b228b997c23094ddfdd71e22e2de6c9310",
		SubaccountID:  "7",
		RecipientID:   "7",
		Nonce:         "12",
		Side:          "buy",
		AssetAddress:  asset,
		SubID:         "1777507200",
		DesiredAmount: "0.002",
		FilledAmount:  "0",
		LimitPrice:    "1391",
		WorstFee:      "0",
		Expiry:        time.Now().Add(time.Hour).Unix(),
		ActionJSON: json.RawMessage(`{
			"subaccount_id":"7",
			"nonce":"12",
			"module":"0x0aae65aaa66fe7f54486cdbd007956d3de611990",
			"data":"` + mustTradeDataHex(asset, "1777507200", "1391000000000000000000", "1000000000000001", true) + `",
			"expiry":"1777507200",
			"owner":"0xc7be60b228b997c23094ddfdd71e22e2de6c9310",
			"signer":"0xc7be60b228b997c23094ddfdd71e22e2de6c9310"
		}`),
		Signature: "0xsig",
	}

	_, err := req.toParams(config.Config{
		CNGNApr2026FutureAssetAddress: asset,
		CNGNApr2026FutureSubID:        "1777507200",
		EnforceActionDataInvariants:   true,
	})
	if err == nil || err.Error() != "action_json.data.desiredAmount is not aligned with normalized desired_amount" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustTradeDataHex(asset string, subID string, limitPrice string, desiredAmount string, isBid bool) string {
	var out []byte
	out = append(out, encodeAddressWord(asset)...)
	out = append(out, encodeUnsignedWord(subID)...)
	out = append(out, encodeSignedWord(limitPrice)...)
	out = append(out, encodeSignedWord(desiredAmount)...)
	out = append(out, encodeUnsignedWord("0")...)
	out = append(out, encodeUnsignedWord("7")...)
	if isBid {
		out = append(out, encodeUnsignedWord("1")...)
	} else {
		out = append(out, encodeUnsignedWord("0")...)
	}
	return "0x" + hex.EncodeToString(out)
}

func encodeAddressWord(address string) []byte {
	raw, _ := hex.DecodeString(address[2:])
	word := make([]byte, 32)
	copy(word[12:], raw)
	return word
}

func encodeUnsignedWord(value string) []byte {
	n, _ := new(big.Int).SetString(value, 10)
	word := make([]byte, 32)
	bytes := n.Bytes()
	copy(word[32-len(bytes):], bytes)
	return word
}

func encodeSignedWord(value string) []byte {
	n, _ := new(big.Int).SetString(value, 10)
	if n.Sign() < 0 {
		mod := new(big.Int).Lsh(big.NewInt(1), 256)
		n = n.Add(n, mod)
	}
	word := make([]byte, 32)
	bytes := n.Bytes()
	copy(word[32-len(bytes):], bytes)
	return word
}
