package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/numofx/matching-backend/internal/config"
	"github.com/numofx/matching-backend/internal/instruments"
	orderrepo "github.com/numofx/matching-backend/internal/orders"
)

func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("MARKETS_SERVICE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("MARKETS_SERVICE_TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func TestHandleBookReturnsSpotBookForUSDCCNGN(t *testing.T) {
	pool := openTestPool(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("api-book-%d", time.Now().UnixNano())
	assetAddress := "0xfeed000000000000000000000000000000000999"

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "delete from active_orders where order_id like $1", suffix+"%")
	})

	insertOrder := `
insert into active_orders (
  order_id, owner_address, signer_address, subaccount_id, recipient_id, nonce, side, asset_address, sub_id,
  desired_amount, filled_amount, limit_price, limit_price_ticks, worst_fee, expiry, action_json, signature, status
) values ($1, $2, $3, 1, 1, $4, $5, $6, $7, $8, '0', $9, $10, '0', $11, '{}'::jsonb, '0xsig', 'active')
`

	expiry := time.Now().Add(time.Hour).Unix()
	if _, err := pool.Exec(ctx, insertOrder, suffix+"-bid", "0xowner", "0xsigner", "1", "buy", assetAddress, "0", "2500000", "1598", "1598", expiry); err != nil {
		t.Fatalf("insert bid: %v", err)
	}
	if _, err := pool.Exec(ctx, insertOrder, suffix+"-ask", "0xowner", "0xsigner", "2", "sell", assetAddress, "0", "3000000", "1602", "1602", expiry); err != nil {
		t.Fatalf("insert ask: %v", err)
	}

	registry := instruments.DefaultRegistry(config.Config{
		CNGNSpotAssetAddress: assetAddress,
	})
	server := NewServer(config.Config{}, pool, registry, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/book?asset_address="+assetAddress+"&sub_id=0", nil)
	rec := httptest.NewRecorder()
	server.handleBook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var response bookResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.MarketPresentation.Market != instruments.CNGNSpotSymbol {
		t.Fatalf("market = %q", response.MarketPresentation.Market)
	}
	if response.MarketPresentation.ContractType != "spot" || response.MarketPresentation.SettlementType != "spot" {
		t.Fatalf("unexpected market metadata %+v", response.MarketPresentation)
	}
	if len(response.Bids) != 1 || len(response.Asks) != 1 {
		t.Fatalf("unexpected book sizes bids=%d asks=%d", len(response.Bids), len(response.Asks))
	}
}

func TestSpotUIOrderEndToEndPathEchoesNormalizedContract(t *testing.T) {
	pool := openTestPool(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("api-spot-e2e-%d", time.Now().UnixNano())
	assetAddress := "0xfeed000000000000000000000000000000000998"

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "delete from trade_fills where taker_order_id like $1 or maker_order_id like $1", suffix+"%")
		_, _ = pool.Exec(ctx, "delete from active_orders where order_id like $1", suffix+"%")
	})

	registry := instruments.DefaultRegistry(config.Config{
		CNGNSpotAssetAddress: assetAddress,
	})
	server := NewServer(config.Config{}, pool, registry, nil)
	repo := orderrepo.NewRepository(pool)

	createPayload := map[string]any{
		"order_id":        suffix + "-ui-buy",
		"owner_address":   "0xabc",
		"signer_address":  "0xabc",
		"subaccount_id":   "10",
		"recipient_id":    "10",
		"nonce":           "1",
		"asset_address":   assetAddress,
		"sub_id":          "0",
		"worst_fee":       "1",
		"expiry":          time.Now().Add(time.Hour).Unix(),
		"order_entry_spec": spotOrderEntrySpec,
		"ui_intent": map[string]any{
			"side":  "buy",
			"price": "1605.25",
			"size":  "100",
		},
		"action_json": map[string]any{
			"subaccount_id": "10",
			"nonce":         "1",
			"module":        "0xtrade",
			"data":          "0xaaa",
			"expiry":        "100",
			"owner":         "0xabc",
			"signer":        "0xabc",
		},
		"signature": "0xsig",
	}

	body, err := json.Marshal(createPayload)
	if err != nil {
		t.Fatalf("marshal create payload: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	server.handleCreateOrder(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created orderResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	if created.Order.Side != orderrepo.SideSell {
		t.Fatalf("engine side = %q", created.Order.Side)
	}
	if created.Order.LimitPrice != "0.000622955925868245" {
		t.Fatalf("engine price = %q", created.Order.LimitPrice)
	}
	if created.Order.DesiredAmount != "160525" {
		t.Fatalf("engine amount = %q", created.Order.DesiredAmount)
	}
	if created.Order.SpotContract == nil {
		t.Fatal("expected spot contract echo on created order")
	}
	if created.Order.SpotContract.UIIntent.Side != "buy" {
		t.Fatalf("ui side = %q", created.Order.SpotContract.UIIntent.Side)
	}
	if created.Order.SpotContract.EngineOrder.Side != "sell" {
		t.Fatalf("echoed engine side = %q", created.Order.SpotContract.EngineOrder.Side)
	}

	bookReq := httptest.NewRequest(http.MethodGet, "/v1/book?asset_address="+assetAddress+"&sub_id=0", nil)
	bookRec := httptest.NewRecorder()
	server.handleBook(bookRec, bookReq)

	if bookRec.Code != http.StatusOK {
		t.Fatalf("book status = %d body=%s", bookRec.Code, bookRec.Body.String())
	}

	var book bookResponse
	if err := json.Unmarshal(bookRec.Body.Bytes(), &book); err != nil {
		t.Fatalf("unmarshal book response: %v", err)
	}
	if len(book.Asks) != 1 || len(book.Bids) != 0 {
		t.Fatalf("unexpected book sizes bids=%d asks=%d", len(book.Bids), len(book.Asks))
	}
	if book.Asks[0].Side != orderrepo.SideSell {
		t.Fatalf("book engine side = %q", book.Asks[0].Side)
	}
	if book.Asks[0].SpotContract == nil || book.Asks[0].SpotContract.UIIntent.Side != "buy" {
		t.Fatalf("book spot echo = %+v", book.Asks[0].SpotContract)
	}

	insertMaker := `
insert into active_orders (
  order_id, owner_address, signer_address, subaccount_id, recipient_id, nonce, side, asset_address, sub_id,
  desired_amount, filled_amount, limit_price, limit_price_ticks, worst_fee, expiry, action_json, signature, status, created_at
) values ($1, $2, $3, 11, 11, $4, $5, $6, $7, $8, '0', $9, $10, '0', $11, '{}'::jsonb, '0xsig', 'active', $12)
`

	if _, err := pool.Exec(
		ctx,
		insertMaker,
		suffix+"-maker",
		"0xmaker",
		"0xmaker",
		"2",
		orderrepo.SideBuy,
		assetAddress,
		"0",
		"160525",
		"0.000622955925868245",
		"622955925868245",
		time.Now().Add(time.Hour).Unix(),
		time.Now().Add(-time.Minute).UTC(),
	); err != nil {
		t.Fatalf("insert maker: %v", err)
	}

	candidate, err := repo.AcquireMatchCandidate(ctx, assetAddress, "0", time.Now().UTC())
	if err != nil {
		t.Fatalf("acquire match candidate: %v", err)
	}
	if candidate == nil {
		t.Fatal("expected match candidate")
	}
	if candidate.Taker.OrderID != suffix+"-ui-buy" {
		t.Fatalf("taker order id = %q", candidate.Taker.OrderID)
	}
	if candidate.Taker.Side != orderrepo.SideSell {
		t.Fatalf("taker engine side = %q", candidate.Taker.Side)
	}

	if err := repo.FinalizeMatchWithPrice(ctx, candidate.Taker.OrderID, candidate.Maker.OrderID, "0.000622955925868245", "160525"); err != nil {
		t.Fatalf("finalize match: %v", err)
	}

	tradesReq := httptest.NewRequest(http.MethodGet, "/v1/trades?asset_address="+assetAddress+"&sub_id=0", nil)
	tradesRec := httptest.NewRecorder()
	server.handleTrades(tradesRec, tradesReq)

	if tradesRec.Code != http.StatusOK {
		t.Fatalf("trades status = %d body=%s", tradesRec.Code, tradesRec.Body.String())
	}

	var trades tradesResponse
	if err := json.Unmarshal(tradesRec.Body.Bytes(), &trades); err != nil {
		t.Fatalf("unmarshal trades response: %v", err)
	}
	if len(trades.Trades) != 1 {
		t.Fatalf("unexpected trade count %d", len(trades.Trades))
	}

	trade := trades.Trades[0]
	if trade.AggressorSide != orderrepo.SideSell {
		t.Fatalf("aggressor side = %q", trade.AggressorSide)
	}
	if trade.SpotContract == nil {
		t.Fatal("expected spot contract echo on trade")
	}
	if trade.SpotContract.UIIntent.Side != "buy" || trade.SpotContract.UIIntent.Price != "1605.25" || trade.SpotContract.UIIntent.Size != "100" {
		t.Fatalf("unexpected ui echo %+v", trade.SpotContract.UIIntent)
	}
	if trade.SpotContract.EngineOrder.Side != "sell" || trade.SpotContract.EngineOrder.Price != "0.000622955925868245" || trade.SpotContract.EngineOrder.Amount != "160525" {
		t.Fatalf("unexpected engine echo %+v", trade.SpotContract.EngineOrder)
	}
	if trade.SpotContract.BalanceDelta.USDC != "+100" || trade.SpotContract.BalanceDelta.CNGN != "-160525" {
		t.Fatalf("unexpected balance delta %+v", trade.SpotContract.BalanceDelta)
	}
}
