package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/numofx/matching-backend/internal/orders"
)

func TestBuildExecutorRequest(t *testing.T) {
	candidate := orders.MatchCandidate{
		Taker: orders.Order{
			OrderID:       "taker-1",
			OwnerAddress:  "0xowner1",
			SignerAddress: "0xsigner1",
			AssetAddress:  "0xabc",
			SubaccountID:  "10",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xowner1","signer":"0xsigner1"}`),
			Signature:     "0xsig1",
			Nonce:         "1",
		},
		Maker: orders.Order{
			OrderID:       "maker-1",
			OwnerAddress:  "0xowner2",
			SignerAddress: "0xsigner2",
			SubaccountID:  "11",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"11","nonce":"2","module":"0xtrade","data":"0xbbb","expiry":"100","owner":"0xowner2","signer":"0xsigner2"}`),
			Signature:     "0xsig2",
			Nonce:         "2",
		},
	}

	req, err := buildExecutorRequest("BTCUSDC-CVXPERP", candidate, "0xfeed", "75", "3")
	if err != nil {
		t.Fatalf("buildExecutorRequest returned error: %v", err)
	}

	if req.TakerOrderID != "taker-1" {
		t.Fatalf("taker order id = %s", req.TakerOrderID)
	}
	if req.MakerOrderID != "maker-1" {
		t.Fatalf("maker order id = %s", req.MakerOrderID)
	}
	if len(req.Actions) != 2 {
		t.Fatalf("actions length = %d", len(req.Actions))
	}
	if req.OrderData.TakerAccount != "10" {
		t.Fatalf("taker account = %s", req.OrderData.TakerAccount)
	}
	if req.OrderData.FillDetails[0].FilledAccount != "11" {
		t.Fatalf("filled account = %s", req.OrderData.FillDetails[0].FilledAccount)
	}
	if req.OrderData.FillDetails[0].Price != "75" {
		t.Fatalf("price = %s", req.OrderData.FillDetails[0].Price)
	}
	if req.OrderData.ManagerData != "0xfeed" {
		t.Fatalf("manager data = %s", req.OrderData.ManagerData)
	}
}

func TestBuildExecutorRequestForBTCVar30Market(t *testing.T) {
	candidate := orders.MatchCandidate{
		Taker: orders.Order{
			OrderID:       "taker-btcvar30",
			OwnerAddress:  "0xowner1",
			SignerAddress: "0xsigner1",
			AssetAddress:  "0x2222222222222222222222222222222222222222",
			SubaccountID:  "501",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"501","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xowner1","signer":"0xsigner1"}`),
			Signature:     "0xsig1",
			Nonce:         "1",
		},
		Maker: orders.Order{
			OrderID:       "maker-btcvar30",
			OwnerAddress:  "0xowner2",
			SignerAddress: "0xsigner2",
			SubaccountID:  "502",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"502","nonce":"2","module":"0xtrade","data":"0xbbb","expiry":"100","owner":"0xowner2","signer":"0xsigner2"}`),
			Signature:     "0xsig2",
			Nonce:         "2",
		},
	}

	req, err := buildExecutorRequest("BTCVAR30-PERP", candidate, "0xfeed", "2728", "3")
	if err != nil {
		t.Fatalf("buildExecutorRequest returned error: %v", err)
	}

	if req.Market != "BTCVAR30-PERP" {
		t.Fatalf("market = %s", req.Market)
	}
	if req.AssetAddress != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("asset address = %s", req.AssetAddress)
	}
	if req.OrderData.FillDetails[0].Price != "2728" {
		t.Fatalf("price = %s", req.OrderData.FillDetails[0].Price)
	}
}

func TestBuildExecutorRequestForSpotMarket(t *testing.T) {
	candidate := orders.MatchCandidate{
		Taker: orders.Order{
			OrderID:       "taker-spot",
			OwnerAddress:  "0xowner1",
			SignerAddress: "0xsigner1",
			AssetAddress:  "0x3333333333333333333333333333333333333333",
			SubaccountID:  "601",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"601","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xowner1","signer":"0xsigner1"}`),
			Signature:     "0xsig1",
			Nonce:         "1",
		},
		Maker: orders.Order{
			OrderID:       "maker-spot",
			OwnerAddress:  "0xowner2",
			SignerAddress: "0xsigner2",
			SubaccountID:  "602",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"602","nonce":"2","module":"0xtrade","data":"0xbbb","expiry":"100","owner":"0xowner2","signer":"0xsigner2"}`),
			Signature:     "0xsig2",
			Nonce:         "2",
		},
	}

	req, err := buildExecutorRequest("USDCcNGN-SPOT", candidate, "0x", "1602", "3000000")
	if err != nil {
		t.Fatalf("buildExecutorRequest returned error: %v", err)
	}

	if req.Market != "USDCcNGN-SPOT" {
		t.Fatalf("market = %s", req.Market)
	}
	if req.AssetAddress != "0x3333333333333333333333333333333333333333" {
		t.Fatalf("asset address = %s", req.AssetAddress)
	}
	if req.OrderData.FillDetails[0].Price != "1602" || req.OrderData.FillDetails[0].AmountFilled != "3000000" {
		t.Fatalf("unexpected fill details %+v", req.OrderData.FillDetails[0])
	}
}

func TestExecutorClientSubmitMatch(t *testing.T) {
	candidate := orders.MatchCandidate{
		Taker: orders.Order{
			OrderID:       "taker-1",
			OwnerAddress:  "0xowner1",
			SignerAddress: "0xsigner1",
			AssetAddress:  "0xabc",
			SubaccountID:  "10",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xowner1","signer":"0xsigner1"}`),
			Signature:     "0xsig1",
			Nonce:         "1",
		},
		Maker: orders.Order{
			OrderID:       "maker-1",
			OwnerAddress:  "0xowner2",
			SignerAddress: "0xsigner2",
			SubaccountID:  "11",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"11","nonce":"2","module":"0xtrade","data":"0xbbb","expiry":"100","owner":"0xowner2","signer":"0xsigner2"}`),
			Signature:     "0xsig2",
			Nonce:         "2",
		},
	}

	client := &ExecutorClient{
		url:         "https://executor.test/submit",
		managerData: "0xfeed",
		httpClient: &http.Client{
			Timeout: time.Second,
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				defer r.Body.Close()
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}

				var req ExecutorRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("unmarshal request: %v", err)
				}

				if req.OrderData.TakerAccount != "10" {
					t.Fatalf("taker account = %s", req.OrderData.TakerAccount)
				}
				if req.OrderData.FillDetails[0].FilledAccount != "11" {
					t.Fatalf("filled account = %s", req.OrderData.FillDetails[0].FilledAccount)
				}
				if req.OrderData.ManagerData != "0xfeed" {
					t.Fatalf("manager data = %s", req.OrderData.ManagerData)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewBufferString(`{"accepted":true,"tx_hash":"0xdeadbeef"}`)),
				}, nil
			}),
		},
	}

	resp, err := client.SubmitMatch(context.Background(), candidate, "75", "3")
	if err != nil {
		t.Fatalf("SubmitMatch returned error: %v", err)
	}
	if !resp.Accepted {
		t.Fatal("expected accepted response")
	}
	if resp.TxHash != "0xdeadbeef" {
		t.Fatalf("tx hash = %s", resp.TxHash)
	}
}

func TestBuildExecutorRequestRejectsActionOwnerMismatch(t *testing.T) {
	candidate := orders.MatchCandidate{
		Taker: orders.Order{
			OrderID:       "taker-1",
			OwnerAddress:  "0xowner1",
			SignerAddress: "0xsigner1",
			AssetAddress:  "0xabc",
			SubaccountID:  "10",
			Nonce:         "1",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xwrong","signer":"0xsigner1"}`),
			Signature:     "0xsig1",
		},
		Maker: orders.Order{
			OrderID:       "maker-1",
			OwnerAddress:  "0xowner2",
			SignerAddress: "0xsigner2",
			SubaccountID:  "11",
			Nonce:         "2",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"11","nonce":"2","module":"0xtrade","data":"0xbbb","expiry":"100","owner":"0xowner2","signer":"0xsigner2"}`),
			Signature:     "0xsig2",
		},
	}

	_, err := buildExecutorRequest("BTCUSDC-CVXPERP", candidate, "0x", "75", "3")
	if err == nil || err.Error() != "parse taker action_json: owner mismatch" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildExecutorRequestDefaultsEmptyManagerData(t *testing.T) {
	candidate := orders.MatchCandidate{
		Taker: orders.Order{
			OrderID:       "taker-1",
			OwnerAddress:  "0xowner1",
			SignerAddress: "0xsigner1",
			AssetAddress:  "0xabc",
			SubaccountID:  "10",
			Nonce:         "1",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"10","nonce":"1","module":"0xtrade","data":"0xaaa","expiry":"100","owner":"0xowner1","signer":"0xsigner1"}`),
			Signature:     "0xsig1",
		},
		Maker: orders.Order{
			OrderID:       "maker-1",
			OwnerAddress:  "0xowner2",
			SignerAddress: "0xsigner2",
			SubaccountID:  "11",
			Nonce:         "2",
			ActionJSON:    json.RawMessage(`{"subaccount_id":"11","nonce":"2","module":"0xtrade","data":"0xbbb","expiry":"100","owner":"0xowner2","signer":"0xsigner2"}`),
			Signature:     "0xsig2",
		},
	}

	req, err := buildExecutorRequest("BTCUSDC-CVXPERP", candidate, "", "75", "3")
	if err != nil {
		t.Fatalf("buildExecutorRequest returned error: %v", err)
	}
	if req.OrderData.ManagerData != "0x" {
		t.Fatalf("manager data = %s", req.OrderData.ManagerData)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
