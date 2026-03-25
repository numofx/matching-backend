package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                        string
	APIAddr                       string
	DatabaseURL                   string
	MatcherPollInterval           time.Duration
	ChainID                       string
	MatchingAddress               string
	TradeModuleAddress            string
	BTCPerpAssetAddress           string
	ExecutorURL                   string
	ExecutorManagerData           string
	ExpectedOrderOwner            string
	ExpectedOrderSigner           string
	DeribitBaseURL                string
	DeribitWSURL                  string
	BTCVar30Enabled               bool
	BTCVar30AssetAddress          string
	BTCVar30OraclePollInterval    time.Duration
	BTCVar30OracleStaleAfter      time.Duration
	BTCVar30FundingInterval       time.Duration
	BTCVar30FundingCoeff          float64
	BTCVar30FundingCap            float64
	BTCVar30OracleSigningKey      string
	CNGNSpotAssetAddress          string
	CNGNApr2026FutureAssetAddress string
	CNGNApr2026FutureSubID        string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:                        getenvDefault("APP_ENV", "dev"),
		APIAddr:                       getenvDefault("API_ADDR", ":8080"),
		DatabaseURL:                   os.Getenv("DATABASE_URL"),
		ChainID:                       os.Getenv("CHAIN_ID"),
		MatchingAddress:               os.Getenv("MATCHING_ADDRESS"),
		TradeModuleAddress:            os.Getenv("TRADE_MODULE_ADDRESS"),
		BTCPerpAssetAddress:           os.Getenv("BTC_PERP_ASSET_ADDRESS"),
		ExecutorURL:                   os.Getenv("EXECUTOR_URL"),
		ExecutorManagerData:           "0x",
		ExpectedOrderOwner:            os.Getenv("EXPECTED_ORDER_OWNER"),
		ExpectedOrderSigner:           os.Getenv("EXPECTED_ORDER_SIGNER"),
		DeribitBaseURL:                getenvDefault("DERIBIT_BASE_URL", "https://test.deribit.com/api/v2"),
		DeribitWSURL:                  getenvDefault("DERIBIT_WS_URL", "wss://test.deribit.com/ws/api/v2"),
		BTCVar30Enabled:               getenvBool("BTCVAR30_ENABLED", false),
		BTCVar30AssetAddress:          strings.ToLower(strings.TrimSpace(os.Getenv("BTCVAR30_PERP_ASSET_ADDRESS"))),
		BTCVar30FundingCoeff:          getenvFloatDefault("BTCVAR30_FUNDING_COEFF", 0.10),
		BTCVar30FundingCap:            getenvFloatDefault("BTCVAR30_FUNDING_CAP", 0.05),
		BTCVar30OracleSigningKey:      strings.TrimSpace(os.Getenv("BTCVAR30_ORACLE_SIGNING_KEY")),
		CNGNSpotAssetAddress:          strings.ToLower(strings.TrimSpace(os.Getenv("CNGN_SPOT_ASSET_ADDRESS"))),
		CNGNApr2026FutureAssetAddress: strings.ToLower(strings.TrimSpace(os.Getenv("CNGN_APR30_2026_FUTURE_ASSET_ADDRESS"))),
		CNGNApr2026FutureSubID:        strings.TrimSpace(os.Getenv("CNGN_APR30_2026_FUTURE_SUB_ID")),
	}

	managerData, err := loadExecutorManagerData()
	if err != nil {
		return Config{}, err
	}
	cfg.ExecutorManagerData = managerData

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	pollInterval, err := time.ParseDuration(getenvDefault("MATCHER_POLL_INTERVAL", "250ms"))
	if err != nil {
		return Config{}, fmt.Errorf("parse MATCHER_POLL_INTERVAL: %w", err)
	}
	cfg.MatcherPollInterval = pollInterval

	oraclePollInterval, err := getenvMillisecondsDuration("BTCVAR30_ORACLE_POLL_MS", 60_000)
	if err != nil {
		return Config{}, fmt.Errorf("parse BTCVAR30_ORACLE_POLL_MS: %w", err)
	}
	cfg.BTCVar30OraclePollInterval = oraclePollInterval

	oracleStaleAfter, err := getenvMillisecondsDuration("BTCVAR30_ORACLE_STALE_MS", 180_000)
	if err != nil {
		return Config{}, fmt.Errorf("parse BTCVAR30_ORACLE_STALE_MS: %w", err)
	}
	cfg.BTCVar30OracleStaleAfter = oracleStaleAfter

	fundingInterval, err := getenvMillisecondsDuration("BTCVAR30_FUNDING_INTERVAL_MS", 28_800_000)
	if err != nil {
		return Config{}, fmt.Errorf("parse BTCVAR30_FUNDING_INTERVAL_MS: %w", err)
	}
	cfg.BTCVar30FundingInterval = fundingInterval

	if cfg.BTCVar30Enabled && cfg.BTCVar30AssetAddress == "" {
		return Config{}, fmt.Errorf("BTCVAR30_PERP_ASSET_ADDRESS is required when BTCVAR30_ENABLED=true")
	}

	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvMillisecondsDuration(key string, fallbackMs int64) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return time.Duration(fallbackMs) * time.Millisecond, nil
	}

	ms, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func getenvFloatDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func loadExecutorManagerData() (string, error) {
	if path := strings.TrimSpace(os.Getenv("EXECUTOR_MANAGER_DATA_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read EXECUTOR_MANAGER_DATA_FILE: %w", err)
		}
		return parseExecutorManagerData(data)
	}

	value := strings.TrimSpace(os.Getenv("EXECUTOR_MANAGER_DATA"))
	if value == "" {
		return "0x", nil
	}
	return value, nil
}

func parseExecutorManagerData(data []byte) (string, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "0x", nil
	}

	if strings.HasPrefix(trimmed, "{") {
		var payload struct {
			ManagerData string `json:"manager_data"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return "", fmt.Errorf("parse EXECUTOR_MANAGER_DATA_FILE json: %w", err)
		}
		if strings.TrimSpace(payload.ManagerData) == "" {
			return "", fmt.Errorf("EXECUTOR_MANAGER_DATA_FILE json missing manager_data")
		}
		return strings.TrimSpace(payload.ManagerData), nil
	}
	return trimmed, nil
}
