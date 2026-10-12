#!/usr/bin/env bash
# Vonix Business API — cURL quick reference
# Export your key: export VONIX_API_KEY="vx_live_..."

API="https://api.getvonix.com"
AUTH="Authorization: Bearer $VONIX_API_KEY"

echo "=== Health check ==="
curl -s "$API/health" | jq .

echo ""
echo "=== Balance ==="
curl -s -H "$AUTH" "$API/v1/balance" | jq .

echo ""
echo "=== Accounts ==="
curl -s -H "$AUTH" "$API/v1/accounts" | jq .

echo ""
echo "=== Send Fiat Payment ==="
curl -s -X POST "$API/v1/transfers" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{
    "currency": "USD",
    "amount": "250.00",
    "recipient": {
      "name": "ACME Corp",
      "iban": "GB29NWBK60161331926819"
    },
    "reference": "INV-2024-007",
    "external_id": "order_curl_001"
  }' | jq .

echo ""
echo "=== Get USDT Deposit Address (Ethereum) ==="
curl -s -H "$AUTH" "$API/v1/crypto/deposit-address?currency=USDT&network=ethereum" | jq .

echo ""
echo "=== Get BTC Deposit Address ==="
curl -s -H "$AUTH" "$API/v1/crypto/deposit-address?currency=BTC&network=bitcoin" | jq .

echo ""
echo "=== Send USDT ==="
curl -s -X POST "$API/v1/crypto/send" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{
    "currency": "USDT",
    "network": "ethereum",
    "amount": "100.000000",
    "address": "0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt7Uv",
    "external_id": "payout_001"
  }' | jq .

echo ""
echo "=== Register Webhook ==="
curl -s -X POST "$API/v1/webhooks" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://yourapp.com/webhooks/vonix",
    "events": ["transfer.created", "transfer.completed", "deposit.confirmed"]
  }' | jq .

echo ""
echo "=== Create API Key ==="
curl -s -X POST "$API/v1/api-keys" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Key",
    "scopes": ["accounts:read", "transfers:write", "crypto:write", "webhooks:write"]
  }' | jq .
