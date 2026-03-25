package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/numofx/matching-backend/internal/config"
	"github.com/numofx/matching-backend/internal/instruments"
	btcvar30instrument "github.com/numofx/matching-backend/internal/instruments/btcvar30"
	"github.com/numofx/matching-backend/internal/orders"
	"github.com/numofx/matching-backend/internal/pricing"
)

type createOrderRequest struct {
	OrderID       string          `json:"order_id"`
	OwnerAddress  string          `json:"owner_address"`
	SignerAddress string          `json:"signer_address"`
	SubaccountID  string          `json:"subaccount_id"`
	RecipientID   string          `json:"recipient_id"`
	Nonce         string          `json:"nonce"`
	Side          string          `json:"side"`
	AssetAddress  string          `json:"asset_address"`
	SubID         string          `json:"sub_id"`
	DesiredAmount string          `json:"desired_amount"`
	FilledAmount  string          `json:"filled_amount"`
	LimitPrice    string          `json:"limit_price"`
	WorstFee      string          `json:"worst_fee"`
	Expiry        int64           `json:"expiry"`
	OrderEntrySpec string           `json:"order_entry_spec,omitempty"`
	UIIntent       *spotOrderIntent `json:"ui_intent,omitempty"`
	ActionJSON    json.RawMessage `json:"action_json"`
	Signature     string          `json:"signature"`
}

func (r createOrderRequest) toParams(cfg config.Config) (orders.CreateOrderParams, error) {
	if r.OrderID == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("order_id is required")
	}
	if r.OwnerAddress == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("owner_address is required")
	}
	if r.SignerAddress == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("signer_address is required")
	}
	if r.SubaccountID == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("subaccount_id is required")
	}
	if r.RecipientID == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("recipient_id is required")
	}
	if r.Nonce == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("nonce is required")
	}
	if r.AssetAddress == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("asset_address is required")
	}
	if r.DesiredAmount == "" && r.UIIntent == nil {
		return orders.CreateOrderParams{}, fmt.Errorf("desired_amount is required")
	}
	if r.LimitPrice == "" && r.UIIntent == nil {
		return orders.CreateOrderParams{}, fmt.Errorf("limit_price is required")
	}
	if r.WorstFee == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("worst_fee is required")
	}
	if r.Signature == "" {
		return orders.CreateOrderParams{}, fmt.Errorf("signature is required")
	}
	if r.Expiry <= time.Now().Unix() {
		return orders.CreateOrderParams{}, fmt.Errorf("expiry must be in the future")
	}
	if !json.Valid(r.ActionJSON) {
		return orders.CreateOrderParams{}, fmt.Errorf("action_json must be valid JSON")
	}

	ownerAddress := strings.ToLower(r.OwnerAddress)
	signerAddress := strings.ToLower(r.SignerAddress)

	if expectedOwner := strings.ToLower(strings.TrimSpace(cfg.ExpectedOrderOwner)); expectedOwner != "" && ownerAddress != expectedOwner {
		return orders.CreateOrderParams{}, fmt.Errorf("owner_address must match configured expected owner")
	}
	if expectedSigner := strings.ToLower(strings.TrimSpace(cfg.ExpectedOrderSigner)); expectedSigner != "" && signerAddress != expectedSigner {
		return orders.CreateOrderParams{}, fmt.Errorf("signer_address must match configured expected signer")
	}

	side := orders.Side(strings.ToLower(r.Side))
	if side != "" && side != orders.SideBuy && side != orders.SideSell {
		return orders.CreateOrderParams{}, fmt.Errorf("side must be 'buy' or 'sell'")
	}

	assetAddress := strings.ToLower(r.AssetAddress)
	subID := r.SubID
	if subID == "" {
		subID = "0"
	}

	if !isAllowedInstrument(cfg, assetAddress, subID) {
		return orders.CreateOrderParams{}, fmt.Errorf("asset_address must match a configured instrument")
	}
	registry := instruments.DefaultRegistry(cfg)
	instrument, ok := registry.ByAssetAndSubID(assetAddress, subID)
	if !ok {
		instrument = instruments.Metadata{
			Symbol:         assetAddress,
			AssetAddress:   assetAddress,
			SubID:          subID,
			TickSize:       "1",
			QuotePrecision: 8,
		}
	}
	if isSpotContractInstrument(instrument) {
		var err error
		side, r.LimitPrice, r.DesiredAmount, err = validateOrTranslateSpotUIIntent(r.OrderEntrySpec, r.UIIntent, side, r.LimitPrice, r.DesiredAmount)
		if err != nil {
			return orders.CreateOrderParams{}, err
		}
		if side != orders.SideBuy && side != orders.SideSell {
			return orders.CreateOrderParams{}, fmt.Errorf("side must be 'buy' or 'sell'")
		}
	} else if r.OrderEntrySpec != "" || r.UIIntent != nil {
		return orders.CreateOrderParams{}, fmt.Errorf("order_entry_spec and ui_intent are only supported for the spot usdc/cngn contract")
	}
	converter, err := pricing.NewConverter(instrument)
	if err != nil {
		return orders.CreateOrderParams{}, err
	}
	limitPriceTicks, normalizedPrice, err := converter.Parse(r.LimitPrice)
	if err != nil {
		return orders.CreateOrderParams{}, err
	}
	if instrument.Symbol == btcvar30instrument.Symbol {
		if err := validateBTCVar30VariancePrice(normalizedPrice); err != nil {
			return orders.CreateOrderParams{}, err
		}
	}
	filledAmount := r.FilledAmount
	if filledAmount == "" {
		filledAmount = "0"
	}

	if err := validateActionJSON(r.ActionJSON, ownerAddress, signerAddress, r.SubaccountID, r.Nonce); err != nil {
		return orders.CreateOrderParams{}, err
	}

	return orders.CreateOrderParams{
		OrderID:         r.OrderID,
		OwnerAddress:    ownerAddress,
		SignerAddress:   signerAddress,
		SubaccountID:    r.SubaccountID,
		RecipientID:     r.RecipientID,
		Nonce:           r.Nonce,
		Side:            side,
		AssetAddress:    assetAddress,
		SubID:           subID,
		DesiredAmount:   r.DesiredAmount,
		FilledAmount:    filledAmount,
		LimitPrice:      normalizedPrice,
		LimitPriceTicks: limitPriceTicks,
		WorstFee:        r.WorstFee,
		Expiry:          r.Expiry,
		ActionJSON:      r.ActionJSON,
		Signature:       r.Signature,
	}, nil
}

func validateBTCVar30VariancePrice(price string) error {
	value, err := pricing.ParseVarianceFloat64(price)
	if err != nil {
		return err
	}

	if value > 5.0 || value < btcvar30instrument.MinVariancePrice || value > btcvar30instrument.MaxVariancePrice {
		return fmt.Errorf("BTCVAR30 prices are variance, not volatility. Example: 0.25 = 50%% vol")
	}
	return nil
}

type cancelOrderRequest struct {
	OwnerAddress string `json:"owner_address"`
	Nonce        string `json:"nonce"`
}

func (r cancelOrderRequest) validate() error {
	if r.OwnerAddress == "" {
		return fmt.Errorf("owner_address is required")
	}
	if r.Nonce == "" {
		return fmt.Errorf("nonce is required")
	}
	return nil
}

func isAllowedInstrument(cfg config.Config, assetAddress string, subID string) bool {
	registry := instruments.DefaultRegistry(cfg)
	if len(registry.Enabled()) == 0 {
		return true
	}
	_, ok := registry.ByAssetAndSubID(assetAddress, subID)
	return ok
}

func validateActionJSON(raw json.RawMessage, ownerAddress string, signerAddress string, subaccountID string, nonce string) error {
	var action struct {
		SubaccountID string `json:"subaccount_id"`
		Nonce        string `json:"nonce"`
		Owner        string `json:"owner"`
		Signer       string `json:"signer"`
	}

	if err := json.Unmarshal(raw, &action); err != nil {
		return fmt.Errorf("parse action_json: %w", err)
	}
	if action.SubaccountID != subaccountID {
		return fmt.Errorf("action_json.subaccount_id must match subaccount_id")
	}
	if action.Nonce != nonce {
		return fmt.Errorf("action_json.nonce must match nonce")
	}
	if strings.ToLower(action.Owner) != ownerAddress {
		return fmt.Errorf("action_json.owner must match owner_address")
	}
	if strings.ToLower(action.Signer) != signerAddress {
		return fmt.Errorf("action_json.signer must match signer_address")
	}
	return nil
}
