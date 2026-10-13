package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

const maxBodyBytes = 64 * 1024 // 64 KB

func main() {
	godotenv.Load()

	keyStore := auth.NewAPIKeyStore()
	webhookStore := webhook.NewStore()

	if os.Getenv("ENV") != "production" {
		seedTestKey(keyStore)
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(bodyLimit(maxBodyBytes))
	r.Use(corsMiddleware)
	r.Use(securityHeaders)

	// 100 req/min per IP
	r.Use(httprate.LimitByIP(100, time.Minute))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","api_version":"v1"}`)
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://docs.getvonix.com/api", http.StatusFound)
	})

	// Public endpoint: supported assets
	r.Get("/v1/crypto/assets", handlers.ListSupportedAssets)

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(keyStore))

		transferH := &handlers.TransferHandler{Webhooks: webhookStore}
		cryptoH := &handlers.CryptoHandler{Webhooks: webhookStore}
		webhookH := &handlers.WebhookHandler{Store: webhookStore}
		apiKeyH := &handlers.APIKeyHandler{Store: keyStore}

		r.With(auth.RequireScope(models.ScopeRead)).Get("/v1/accounts", handlers.ListAccounts)
		r.With(auth.RequireScope(models.ScopeRead)).Get("/v1/balance", handlers.GetBalance)

		r.With(auth.RequireScope(models.ScopeRead)).Get("/v1/transfers", transferH.ListTransactions)
		r.With(auth.RequireScope(models.ScopeTransfer)).Post("/v1/transfers", transferH.CreateFiatTransfer)

		r.With(auth.RequireScope(models.ScopeRead)).Get("/v1/crypto/deposit-address", cryptoH.GetDepositAddress)
		r.With(auth.RequireScope(models.ScopeCrypto)).Post("/v1/crypto/send", cryptoH.SendCrypto)

		r.With(auth.RequireScope(models.ScopeWebhooks)).Post("/v1/webhooks", webhookH.Create)
		r.With(auth.RequireScope(models.ScopeRead)).Get("/v1/webhooks", webhookH.List)
		r.With(auth.RequireScope(models.ScopeWebhooks)).Delete("/v1/webhooks/{webhookID}", webhookH.Delete)

		r.With(auth.RequireScope(models.ScopeRead)).Get("/v1/api-keys", apiKeyH.List)
		r.With(auth.RequireScope(models.ScopeAPIKeys)).Post("/v1/api-keys", apiKeyH.Create)
		r.With(auth.RequireScope(models.ScopeAPIKeys)).Delete("/v1/api-keys/{keyID}", apiKeyH.Revoke)
	})

	// 404 for anything not matched (avoid path traversal info leaks)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"Not Found","code":"not_found","message":"The requested endpoint does not exist."}`)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("Vonix Business API starting", "port", port, "env", os.Getenv("ENV"))
	if err := http.ListenAndServe(":"+port, r); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func seedTestKey(store *auth.APIKeyStore) {
	key, plain, err := store.Generate("biz_test_01", models.CreateAPIKeyRequest{
		Name:   "Test Key",
		Scopes: []models.APIKeyScope{models.ScopeRead, models.ScopeTransfer, models.ScopeCrypto, models.ScopeWebhooks, models.ScopeAPIKeys},
	})
	if err != nil {
		return
	}
	_ = key
	slog.Info("─── TEST API KEY (dev only) ───", "key", plain)
}

// bodyLimit caps request body size to prevent DoS.
func bodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware restricts cross-origin access to known origins.
// For a public API, allow all origins on GET but tighten on mutations.
func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := getAllowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.Header().Set("Vary", "Origin")
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
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Vonix-API-Version", "v1")
		next.ServeHTTP(w, r)
	})
}

func slogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration", time.Since(start),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

func getAllowedOrigins() []string {
	env := os.Getenv("CORS_ORIGINS")
	if env != "" {
		return strings.Split(env, ",")
	}
	return []string{
		"https://app.getvonix.com",
		"https://business.getvonix.com",
		"https://getvonix.com",
	}
}

func isAllowedOrigin(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, o := range allowed {
		if strings.TrimSpace(o) == origin {
			return true
		}
	}
	return false
}
