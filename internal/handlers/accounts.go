package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/vonix/api/internal/auth"
	"github.com/vonix/api/internal/models"
)

func ListAccounts(w http.ResponseWriter, r *http.Request) {
	bid := auth.BusinessID(r)

	// In production: query your database for accounts by business_id
	accounts := []models.Account{
		{
			ID:         "acc_01j9fiat_usd",
			BusinessID: bid,
			Type:       models.AccountTypeFiat,
			Currency:   models.CurrencyUSD,
			IBAN:       "GB82VONX00012312345678",
			Balance: models.Balance{
				Currency:  models.CurrencyUSD,
				Available: "24850.00",
				Pending:   "1200.00",
				Total:     "26050.00",
			},
			CreatedAt: time.Now().AddDate(0, -3, 0),
		},
		{
			ID:         "acc_01j9crypto_usdt",
			BusinessID: bid,
			Type:       models.AccountTypeCrypto,
			Currency:   models.CurrencyUSDT,
			Network:    "ethereum",
			Balance: models.Balance{
				Currency:  models.CurrencyUSDT,
				Available: "8400.000000",
				Pending:   "0.000000",
				Total:     "8400.000000",
			},
			CreatedAt: time.Now().AddDate(0, -2, 0),
		},
		{
			ID:         "acc_01j9crypto_btc",
			BusinessID: bid,
			Type:       models.AccountTypeCrypto,
			Currency:   models.CurrencyBTC,
			Network:    "bitcoin",
			Balance: models.Balance{
				Currency:  models.CurrencyBTC,
				Available: "0.14820000",
				Pending:   "0.00000000",
				Total:     "0.14820000",
			},
			CreatedAt: time.Now().AddDate(0, -2, 0),
		},
	}

	respond(w, http.StatusOK, models.PaginatedResponse[models.Account]{
		Data:       accounts,
		TotalCount: len(accounts),
		HasMore:    false,
	})
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	bid := auth.BusinessID(r)

	// In production: query balances for the given business
	balances := []models.Balance{
		{Currency: models.CurrencyUSD, Available: "24850.00", Pending: "1200.00", Total: "26050.00"},
		{Currency: models.CurrencyGBP, Available: "0.00", Pending: "0.00", Total: "0.00"},
		{Currency: models.CurrencyUSDT, Available: "8400.000000", Pending: "0.000000", Total: "8400.000000"},
		{Currency: models.CurrencyBTC, Available: "0.14820000", Pending: "0.00000000", Total: "0.14820000"},
		{Currency: models.CurrencyETH, Available: "3.220000000000000000", Pending: "0.000000000000000000", Total: "3.220000000000000000"},
	}

	_ = bid
	respond(w, http.StatusOK, map[string]any{
		"business_id": bid,
		"balances":    balances,
		"as_of":       time.Now().UTC(),
	})
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
