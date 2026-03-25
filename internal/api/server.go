package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/numofx/matching-backend/internal/config"
	"github.com/numofx/matching-backend/internal/instruments"
	oraclemodule "github.com/numofx/matching-backend/internal/oracles/btcvar30"
	"github.com/numofx/matching-backend/internal/orders"
	"github.com/numofx/matching-backend/internal/pricing"
)

type oracleReader interface {
	Latest() (oraclemodule.Payload, bool)
	History(ctx context.Context, limit int) ([]oraclemodule.Payload, error)
}

type Server struct {
	cfg         config.Config
	pool        *pgxpool.Pool
	orders      *orders.Repository
	instruments *instruments.Registry
	oracle      oracleReader
}

type marketPresentation struct {
	Market             string `json:"market"`
	ContractType       string `json:"contract_type,omitempty"`
	SettlementType     string `json:"settlement_type,omitempty"`
	BaseAssetSymbol    string `json:"base_asset_symbol,omitempty"`
	QuoteAssetSymbol   string `json:"quote_asset_symbol,omitempty"`
	ExpiryTimestamp    int64  `json:"expiry_timestamp,omitempty"`
	LastTradeTimestamp int64  `json:"last_trade_timestamp,omitempty"`
	PriceSemantics     string `json:"price_semantics,omitempty"`
	DisplaySemantics   string `json:"display_semantics,omitempty"`
	DisplayName        string `json:"display_name,omitempty"`
	DisplayLabel       string `json:"display_label,omitempty"`
	TickSize           string `json:"tick_size,omitempty"`
	SettlementNote     string `json:"settlement_note,omitempty"`
	PricingModel       string `json:"pricing_model,omitempty"`
	DisplayPriceKind   string `json:"display_price_kind,omitempty"`
	AssetAddress       string `json:"asset_address,omitempty"`
	SubID              string `json:"sub_id,omitempty"`
	OrderEntrySpec     string `json:"order_entry_spec,omitempty"`
	UIPriceUnit        string `json:"ui_price_unit,omitempty"`
	UISizeUnit         string `json:"ui_size_unit,omitempty"`
	UISideMeaning      string `json:"ui_side_meaning,omitempty"`
	EnginePriceUnit    string `json:"engine_price_unit,omitempty"`
	EngineAmountUnit   string `json:"engine_amount_unit,omitempty"`
	EngineSidePolicy   string `json:"engine_side_policy,omitempty"`
	UIPriceToEngine    string `json:"ui_price_to_engine,omitempty"`
	UISizeToEngine     string `json:"ui_size_to_engine,omitempty"`
}

type presentedOrder struct {
	OrderID          string          `json:"order_id"`
	OwnerAddress     string          `json:"owner_address"`
	SignerAddress    string          `json:"signer_address"`
	SubaccountID     string          `json:"subaccount_id"`
	RecipientID      string          `json:"recipient_id"`
	Nonce            string          `json:"nonce"`
	Side             orders.Side     `json:"side"`
	AssetAddress     string          `json:"asset_address"`
	SubID            string          `json:"sub_id"`
	DesiredAmount    string          `json:"desired_amount"`
	FilledAmount     string          `json:"filled_amount"`
	LimitPrice       string          `json:"limit_price"`
	WorstFee         string          `json:"worst_fee"`
	Expiry           int64           `json:"expiry"`
	ActionJSON       json.RawMessage `json:"action_json"`
	Signature        string          `json:"signature"`
	Status           orders.Status   `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	Market           string          `json:"market,omitempty"`
	ContractType     string          `json:"contract_type,omitempty"`
	SettlementType   string          `json:"settlement_type,omitempty"`
	BaseAssetSymbol  string          `json:"base_asset_symbol,omitempty"`
	QuoteAssetSymbol string          `json:"quote_asset_symbol,omitempty"`
	ExpiryTimestamp  int64           `json:"expiry_timestamp,omitempty"`
	VariancePrice    float64         `json:"variance_price,omitempty"`
	VolPercent       float64         `json:"vol_percent,omitempty"`
	PriceSemantics   string          `json:"price_semantics,omitempty"`
	DisplayName      string          `json:"display_name,omitempty"`
	DisplayLabel     string          `json:"display_label,omitempty"`
	DisplaySemantic  string          `json:"display_semantics,omitempty"`
	TickSize         string          `json:"tick_size,omitempty"`
	SpotContract     *spotOrderContractEcho `json:"spot_contract,omitempty"`
}

type orderResponse struct {
	Order presentedOrder `json:"order"`
}

type bookResponse struct {
	MarketPresentation marketPresentation `json:"market_presentation"`
	Bids               []presentedOrder   `json:"bids"`
	Asks               []presentedOrder   `json:"asks"`
}

type presentedTrade struct {
	TradeID        int64       `json:"trade_id"`
	AssetAddress   string      `json:"asset_address"`
	SubID          string      `json:"sub_id"`
	Price          string      `json:"price"`
	Size           string      `json:"size"`
	AggressorSide  orders.Side `json:"aggressor_side"`
	TakerOrderID   string      `json:"taker_order_id,omitempty"`
	MakerOrderID   string      `json:"maker_order_id,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	Market         string      `json:"market,omitempty"`
	ContractType   string      `json:"contract_type,omitempty"`
	SettlementType string      `json:"settlement_type,omitempty"`
	SpotContract   *spotOrderContractEcho `json:"spot_contract,omitempty"`
}

type presentedTradeStats struct {
	Change string `json:"change,omitempty"`
	High   string `json:"high,omitempty"`
	Last   string `json:"last,omitempty"`
	Low    string `json:"low,omitempty"`
	Volume string `json:"volume,omitempty"`
}

type tradesResponse struct {
	MarketPresentation marketPresentation  `json:"market_presentation"`
	Stats              presentedTradeStats `json:"stats_24h"`
	Trades             []presentedTrade    `json:"trades"`
	NextBeforeTradeID  int64               `json:"next_before_trade_id,omitempty"`
}

type oraclePayloadResponse struct {
	Symbol             string    `json:"symbol"`
	Market             string    `json:"market"`
	PriceSemantics     string    `json:"price_semantics"`
	DisplaySemantics   string    `json:"display_semantics"`
	DisplayName        string    `json:"display_name"`
	TickSize           string    `json:"tick_size"`
	Source             string    `json:"source"`
	Timestamp          time.Time `json:"timestamp"`
	Vol30D             float64   `json:"vol_30d"`
	Variance30D        float64   `json:"variance_30d"`
	VolPercent         float64   `json:"vol_percent"`
	MethodologyVersion string    `json:"methodology_version"`
	Signature          string    `json:"signature,omitempty"`
	Stale              bool      `json:"stale,omitempty"`
}

type oracleHistoryResponse struct {
	Symbol           string                  `json:"symbol"`
	Market           string                  `json:"market"`
	PriceSemantics   string                  `json:"price_semantics"`
	DisplaySemantics string                  `json:"display_semantics"`
	DisplayName      string                  `json:"display_name"`
	TickSize         string                  `json:"tick_size"`
	History          []oraclePayloadResponse `json:"history"`
}

func NewServer(cfg config.Config, pool *pgxpool.Pool, registry *instruments.Registry, oracle oracleReader) *Server {
	return &Server{
		cfg:         cfg,
		pool:        pool,
		orders:      orders.NewRepository(pool),
		instruments: registry,
		oracle:      oracle,
	}
}

func (s *Server) Run() error {
	router := chi.NewRouter()
	router.Get("/healthz", s.handleHealth)
	router.Get("/v1/markets", s.handleMarkets)
	router.Get("/v1/book", s.handleBook)
	router.Get("/v1/trades", s.handleTrades)
	router.Post("/v1/orders", s.handleCreateOrder)
	router.Post("/v1/orders/cancel", s.handleCancelOrder)
	router.Get("/oracle/btcvar30/latest", s.handleBTCVar30Latest)
	router.Get("/oracle/btcvar30/history", s.handleBTCVar30History)

	slog.Info("api listening", "addr", s.cfg.APIAddr)
	return http.ListenAndServe(s.cfg.APIAddr, router)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMarkets(w http.ResponseWriter, _ *http.Request) {
	if s.instruments == nil {
		writeJSON(w, http.StatusOK, []marketPresentation{})
		return
	}

	items := s.instruments.Enabled()
	sort.Slice(items, func(i, j int) bool {
		return items[i].Symbol < items[j].Symbol
	})

	response := make([]marketPresentation, 0, len(items))
	for _, item := range items {
		response = append(response, presentMarket(item))
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleBook(w http.ResponseWriter, r *http.Request) {
	market := s.resolveMarket(r)
	if market.AssetAddress == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown market"})
		return
	}

	bids, asks, err := s.orders.ListBook(r.Context(), strings.ToLower(market.AssetAddress), market.SubID, 25)
	if err != nil {
		slog.Error("list book", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load book"})
		return
	}

	writeJSON(w, http.StatusOK, bookResponse{
		MarketPresentation: presentMarket(market),
		Bids:               presentOrders(bids, market),
		Asks:               presentOrders(asks, market),
	})
}

func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	market := s.resolveMarket(r)
	if market.AssetAddress == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown market"})
		return
	}

	limit := int32(50)
	beforeTradeID := int64(0)
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be between 1 and 100"})
			return
		}
		limit = int32(parsed)
	}
	if rawBefore := strings.TrimSpace(r.URL.Query().Get("before_trade_id")); rawBefore != "" {
		parsed, err := strconv.ParseInt(rawBefore, 10, 64)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "before_trade_id must be a positive integer"})
			return
		}
		beforeTradeID = parsed
	}

	items, err := s.orders.ListTrades(r.Context(), strings.ToLower(market.AssetAddress), market.SubID, beforeTradeID, limit)
	if err != nil {
		slog.Error("list trades", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load trades"})
		return
	}
	stats, err := s.orders.GetTradeStats24h(r.Context(), strings.ToLower(market.AssetAddress), market.SubID)
	if err != nil {
		slog.Error("trade stats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load trades"})
		return
	}

	nextBeforeTradeID := int64(0)
	if len(items) == int(limit) {
		nextBeforeTradeID = items[len(items)-1].TradeID
	}

	writeJSON(w, http.StatusOK, tradesResponse{
		MarketPresentation: presentMarket(market),
		Stats:              presentTradeStats(stats),
		Trades:             presentTrades(items, market),
		NextBeforeTradeID:  nextBeforeTradeID,
	})
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	params, err := req.toParams(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	order, err := s.orders.Create(r.Context(), params)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "duplicate order") {
			statusCode = http.StatusConflict
		}
		slog.Error("create order", "error", err)
		writeJSON(w, statusCode, map[string]string{"error": err.Error()})
		return
	}

	instrument, _ := s.instruments.ByAssetAndSubID(strings.ToLower(order.AssetAddress), order.SubID)
	writeJSON(w, http.StatusCreated, orderResponse{Order: presentOrder(order, instrument)})
}

func (s *Server) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	var req cancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	order, err := s.orders.CancelByOwnerNonce(r.Context(), orders.CancelOrderParams{
		OwnerAddress: strings.ToLower(req.OwnerAddress),
		Nonce:        req.Nonce,
	})
	if err != nil {
		if errors.Is(err, orders.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "active order not found"})
			return
		}
		slog.Error("cancel order", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel order"})
		return
	}

	instrument, _ := s.instruments.ByAssetAndSubID(strings.ToLower(order.AssetAddress), order.SubID)
	writeJSON(w, http.StatusOK, orderResponse{Order: presentOrder(order, instrument)})
}

func (s *Server) handleBTCVar30Latest(w http.ResponseWriter, _ *http.Request) {
	if s.oracle == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "btcvar30 oracle is not configured"})
		return
	}

	payload, ok := s.oracle.Latest()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "btcvar30 oracle has no data"})
		return
	}

	writeJSON(w, http.StatusOK, decorateBTCVar30OraclePayload(payload))
}

func (s *Server) handleBTCVar30History(w http.ResponseWriter, r *http.Request) {
	if s.oracle == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "btcvar30 oracle is not configured"})
		return
	}

	limit := 100
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 1000 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be between 1 and 1000"})
			return
		}
		limit = parsed
	}

	items, err := s.oracle.History(r.Context(), limit)
	if err != nil {
		slog.Error("load btcvar30 history", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load btcvar30 history"})
		return
	}

	writeJSON(w, http.StatusOK, oracleHistoryResponse{
		Symbol:           oraclemodule.Symbol,
		Market:           "BTCVAR30-PERP",
		PriceSemantics:   instruments.PricingModelVariance,
		DisplaySemantics: instruments.DisplayPriceVolPercent,
		DisplayName:      "BTC 30D Implied Volatility Perpetual",
		TickSize:         "0.0001",
		History:          decorateBTCVar30OracleHistory(items),
	})
}

func (s *Server) resolveMarket(r *http.Request) instruments.Metadata {
	if s.instruments == nil {
		return instruments.Metadata{}
	}

	if symbol := strings.TrimSpace(r.URL.Query().Get("symbol")); symbol != "" {
		if item, ok := s.instruments.BySymbol(symbol); ok {
			return item
		}
	}

	if assetAddress := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("asset_address"))); assetAddress != "" {
		subID := strings.TrimSpace(r.URL.Query().Get("sub_id"))
		if subID == "" {
			subID = "0"
		}
		if item, ok := s.instruments.ByAssetAndSubID(assetAddress, subID); ok {
			return item
		}
	}

	if item, ok := s.instruments.BySymbol(instruments.CNGNSpotSymbol); ok {
		return item
	}
	if item, ok := s.instruments.BySymbol(instruments.BTCConvexPerpSymbol); ok {
		return item
	}
	return instruments.Metadata{}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func presentOrders(items []orders.Order, instrument instruments.Metadata) []presentedOrder {
	if len(items) == 0 {
		return nil
	}

	presented := make([]presentedOrder, 0, len(items))
	for _, item := range items {
		presented = append(presented, presentOrder(item, instrument))
	}
	return presented
}

func presentTrades(items []orders.TradeFill, instrument instruments.Metadata) []presentedTrade {
	if len(items) == 0 {
		return nil
	}

	presented := make([]presentedTrade, 0, len(items))
	for _, item := range items {
		spotContract, _ := deriveSpotContractFromTrade(item, instrument)
		presented = append(presented, presentedTrade{
			TradeID:        item.TradeID,
			AssetAddress:   strings.ToLower(item.AssetAddress),
			SubID:          item.SubID,
			Price:          item.Price,
			Size:           item.Size,
			AggressorSide:  item.AggressorSide,
			TakerOrderID:   item.TakerOrderID,
			MakerOrderID:   item.MakerOrderID,
			CreatedAt:      item.CreatedAt,
			Market:         instrument.Symbol,
			ContractType:   instrument.ContractType,
			SettlementType: instrument.SettlementType,
			SpotContract:   spotContract,
		})
	}
	return presented
}

func presentTradeStats(stats orders.TradeStats24h) presentedTradeStats {
	return presentedTradeStats{
		Change: stats.Change,
		High:   stats.High,
		Last:   stats.Last,
		Low:    stats.Low,
		Volume: stats.Volume,
	}
}

func presentOrder(order orders.Order, instrument instruments.Metadata) presentedOrder {
	spotContract, _ := deriveSpotContractFromOrder(order, instrument)
	presented := presentedOrder{
		OrderID:          order.OrderID,
		OwnerAddress:     order.OwnerAddress,
		SignerAddress:    order.SignerAddress,
		SubaccountID:     order.SubaccountID,
		RecipientID:      order.RecipientID,
		Nonce:            order.Nonce,
		Side:             order.Side,
		AssetAddress:     order.AssetAddress,
		SubID:            order.SubID,
		DesiredAmount:    order.DesiredAmount,
		FilledAmount:     order.FilledAmount,
		LimitPrice:       order.LimitPrice,
		WorstFee:         order.WorstFee,
		Expiry:           order.Expiry,
		ActionJSON:       order.ActionJSON,
		Signature:        order.Signature,
		Status:           order.Status,
		CreatedAt:        order.CreatedAt,
		Market:           instrument.Symbol,
		ContractType:     instrument.ContractType,
		SettlementType:   instrument.SettlementType,
		BaseAssetSymbol:  instrument.BaseAssetSymbol,
		QuoteAssetSymbol: instrument.QuoteAssetSymbol,
		ExpiryTimestamp:  instrument.ExpiryTimestamp,
		PriceSemantics:   instrument.PriceSemantics,
		DisplayName:      instrument.DisplayName,
		DisplayLabel:     instrument.DisplayLabel,
		DisplaySemantic:  instrument.DisplaySemantics,
		TickSize:         instrument.TickSize,
		SpotContract:     spotContract,
	}
	if instrument.PricingModel != instruments.PricingModelVariance {
		return presented
	}

	display, err := pricing.VarianceDisplayFromTicks(instrument, order.LimitPriceTicks)
	if err != nil {
		return presented
	}

	presented.VariancePrice = display.VariancePrice
	presented.VolPercent = display.VolPercent
	return presented
}

func presentMarket(market instruments.Metadata) marketPresentation {
	return marketPresentation{
		Market:             market.Symbol,
		ContractType:       market.ContractType,
		SettlementType:     market.SettlementType,
		BaseAssetSymbol:    market.BaseAssetSymbol,
		QuoteAssetSymbol:   market.QuoteAssetSymbol,
		ExpiryTimestamp:    market.ExpiryTimestamp,
		LastTradeTimestamp: market.LastTradeTimestamp,
		PriceSemantics:     market.PriceSemantics,
		DisplaySemantics:   market.DisplaySemantics,
		DisplayName:        market.DisplayName,
		DisplayLabel:       market.DisplayLabel,
		TickSize:           market.TickSize,
		SettlementNote:     market.SettlementNote,
		PricingModel:       market.PricingModel,
		DisplayPriceKind:   market.DisplayPriceKind,
		AssetAddress:       strings.ToLower(market.AssetAddress),
		SubID:              market.SubID,
		OrderEntrySpec:     market.OrderEntrySpec,
		UIPriceUnit:        market.UIPriceUnit,
		UISizeUnit:         market.UISizeUnit,
		UISideMeaning:      market.UISideMeaning,
		EnginePriceUnit:    market.EnginePriceUnit,
		EngineAmountUnit:   market.EngineAmountUnit,
		EngineSidePolicy:   market.EngineSidePolicy,
		UIPriceToEngine:    market.UIPriceToEngine,
		UISizeToEngine:     market.UISizeToEngine,
	}
}

func decorateBTCVar30OraclePayload(payload oraclemodule.Payload) oraclePayloadResponse {
	return oraclePayloadResponse{
		Symbol:             payload.Symbol,
		Market:             "BTCVAR30-PERP",
		PriceSemantics:     instruments.PricingModelVariance,
		DisplaySemantics:   instruments.DisplayPriceVolPercent,
		DisplayName:        "BTC 30D Implied Volatility Perpetual",
		TickSize:           "0.0001",
		Source:             payload.Source,
		Timestamp:          payload.Timestamp,
		Vol30D:             payload.Vol30D,
		Variance30D:        payload.Variance30D,
		VolPercent:         pricing.RoundVolPercent(pricing.VarianceToVolPercent(payload.Variance30D)),
		MethodologyVersion: payload.MethodologyVersion,
		Signature:          payload.Signature,
		Stale:              payload.Stale,
	}
}

func decorateBTCVar30OracleHistory(items []oraclemodule.Payload) []oraclePayloadResponse {
	decorated := make([]oraclePayloadResponse, 0, len(items))
	for _, item := range items {
		decorated = append(decorated, decorateBTCVar30OraclePayload(item))
	}
	return decorated
}
