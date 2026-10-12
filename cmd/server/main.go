package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/joho/godotenv"
	"github.com/vonix/api/internal/auth"
	"github.com/vonix/api/internal/handlers"
	"github.com/vonix/api/internal/models"
	"github.com/vonix/api/internal/webhook"
)

const apiVersion = "v1"

func main() {
	godotenv.Load()

	keyStore := auth.NewAPIKeyStore()
	webhookStore := webhook.NewStore()

	// Seed a default business + API key for quick testing
	if os.Getenv("ENV") != "production" {
		seedTestKey(keyStore)
	}

	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)
	r.Use(securityHeaders)

	// Rate limiting: 100 req/min per IP
	r.Use(httprate.LimitByIP(100, time.Minute))

	// Health check (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":"%s","api_version":"%s"}`, "1.0.0", apiVersion)
	})

	// Public docs redirect
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://docs.getvonix.com/api", http.StatusFound)
	})

	// Supported crypto assets (public)
	r.Get("/v1/crypto/assets", handlers.ListSupportedAssets)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(keyStore))

		transferH := &handlers.TransferHandler{Webhooks: webhookStore}
		cryptoH := &handlers.CryptoHandler{Webhooks: webhookStore}
		webhookH := &handlers.WebhookHandler{Store: webhookStore}
		apiKeyH := &handlers.APIKeyHandler{Store: keyStore}

		// Accounts & balance
		r.With(auth.RequireScope(models.ScopeRead)).
			Get("/v1/accounts", handlers.ListAccounts)
		r.With(auth.RequireScope(models.ScopeRead)).
			Get("/v1/balance", handlers.GetBalance)

		// Fiat transfers
		r.With(auth.RequireScope(models.ScopeRead)).
			Get("/v1/transfers", transferH.ListTransactions)
		r.With(auth.RequireScope(models.ScopeTransfer)).
			Post("/v1/transfers", transferH.CreateFiatTransfer)

		// Crypto
		r.With(auth.RequireScope(models.ScopeRead)).
			Get("/v1/crypto/deposit-address", cryptoH.GetDepositAddress)
		r.With(auth.RequireScope(models.ScopeCrypto)).
			Post("/v1/crypto/send", cryptoH.SendCrypto)

		// Webhooks
		r.With(auth.RequireScope(models.ScopeWebhooks)).
			Post("/v1/webhooks", webhookH.Create)
		r.With(auth.RequireScope(models.ScopeRead)).
			Get("/v1/webhooks", webhookH.List)
		r.With(auth.RequireScope(models.ScopeWebhooks)).
			Delete("/v1/webhooks/{webhookID}", webhookH.Delete)

		// API Keys (self-management)
		r.With(auth.RequireScope(models.ScopeRead)).
			Get("/v1/api-keys", apiKeyH.List)
		r.With(auth.RequireScope(models.ScopeWebhooks)). // admin scope required
			Post("/v1/api-keys", apiKeyH.Create)
		r.With(auth.RequireScope(models.ScopeWebhooks)).
			Delete("/v1/api-keys/{keyID}", apiKeyH.Revoke)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("Vonix Business API starting", "port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func seedTestKey(store *auth.APIKeyStore) {
	key, plain, err := store.Generate("biz_test_01", models.CreateAPIKeyRequest{
		Name:   "Test Key",
		Scopes: []models.APIKeyScope{models.ScopeRead, models.ScopeTransfer, models.ScopeCrypto, models.ScopeWebhooks},
	})
	if err != nil {
		return
	}
	_ = key
	slog.Info("Test API key generated", "key", plain, "business_id", "biz_test_01")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("X-Vonix-API-Version", apiVersion)
		next.ServeHTTP(w, r)
	})
}
