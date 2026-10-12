"""
Vonix Business API — Python examples

pip install requests
"""

import hashlib
import hmac
import json
import os
import time
import requests

BASE_URL = "https://api.getvonix.com"
API_KEY  = os.environ["VONIX_API_KEY"]  # vx_live_...

session = requests.Session()
session.headers.update({
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type":  "application/json",
})


def get_balance() -> dict:
    return session.get(f"{BASE_URL}/v1/balance").json()


def list_accounts() -> dict:
    return session.get(f"{BASE_URL}/v1/accounts").json()


def send_fiat_payment(
    currency: str,
    amount: str,
    recipient_name: str,
    iban: str,
    reference: str = "",
    external_id: str = "",
) -> dict:
    return session.post(f"{BASE_URL}/v1/transfers", json={
        "currency":    currency,
        "amount":      amount,
        "recipient":   {"name": recipient_name, "iban": iban},
        "reference":   reference,
        "external_id": external_id or f"order_{int(time.time())}",
    }).json()


def get_deposit_address(currency: str = "USDT", network: str = "ethereum") -> dict:
    return session.get(
        f"{BASE_URL}/v1/crypto/deposit-address",
        params={"currency": currency, "network": network},
    ).json()


def send_crypto(
    currency: str,
    network: str,
    amount: str,
    address: str,
    external_id: str = "",
) -> dict:
    return session.post(f"{BASE_URL}/v1/crypto/send", json={
        "currency":    currency,
        "network":     network,
        "amount":      amount,
        "address":     address,
        "external_id": external_id,
    }).json()


def register_webhook(url: str, events: list[str]) -> dict:
    resp = session.post(f"{BASE_URL}/v1/webhooks", json={
        "url":    url,
        "events": events,
    }).json()
    print("Store this webhook secret securely:", resp.get("secret"))
    return resp


def create_api_key(name: str, scopes: list[str]) -> dict:
    resp = session.post(f"{BASE_URL}/v1/api-keys", json={
        "name":   name,
        "scopes": scopes,
    }).json()
    print("Store this API key securely:", resp.get("key"))
    return resp


# ──────────────────────────────────────────────
# Webhook signature verification (Flask example)
# ──────────────────────────────────────────────

def verify_webhook_signature(
    secret: str,
    timestamp: str,
    signature: str,
    raw_body: bytes,
) -> bool:
    message = f"{timestamp}.".encode() + raw_body
    expected = "sha256=" + hmac.new(
        secret.encode(), message, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature)


# Flask webhook receiver:
#
# from flask import Flask, request, abort
# app = Flask(__name__)
#
# @app.post("/webhooks/vonix")
# def vonix_webhook():
#     ts  = request.headers["Vonix-Timestamp"]
#     sig = request.headers["Vonix-Signature"]
#     if not verify_webhook_signature(WEBHOOK_SECRET, ts, sig, request.get_data()):
#         abort(401)
#     event = request.json
#     if event["event"] == "transfer.completed":
#         handle_transfer_completed(event["data"])
#     elif event["event"] == "deposit.confirmed":
#         handle_deposit_confirmed(event["data"])
#     return "ok", 200


if __name__ == "__main__":
    print("\n── Balance ─────────────────────────────")
    print(json.dumps(get_balance(), indent=2))

    print("\n── Send Fiat (USD → IBAN) ───────────────")
    print(json.dumps(send_fiat_payment(
        currency       = "USD",
        amount         = "500.00",
        recipient_name = "ACME Ltd",
        iban           = "GB29NWBK60161331926819",
        reference      = "INV-2024-007",
    ), indent=2))

    print("\n── USDT Deposit Address (Ethereum) ──────")
    print(json.dumps(get_deposit_address("USDT", "ethereum"), indent=2))

    print("\n── Send BTC ─────────────────────────────")
    print(json.dumps(send_crypto(
        currency = "BTC",
        network  = "bitcoin",
        amount   = "0.01000000",
        address  = "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
    ), indent=2))
