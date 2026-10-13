// Vonix Business API — Rust examples
// Cargo.toml dependencies:
//   reqwest = { version = "0.12", features = ["json"] }
//   tokio   = { version = "1", features = ["full"] }
//   serde_json = "1"
//   hmac    = "0.12"
//   sha2    = "0.10"
//   hex     = "0.4"

use hmac::{Hmac, Mac};
use sha2::Sha256;
use std::env;
use std::time::{SystemTime, UNIX_EPOCH};

const BASE_URL: &str = "https://api.getvonix.com";

struct VonixClient {
    http:    reqwest::Client,
    api_key: String,
}

impl VonixClient {
    fn new() -> Self {
        Self {
            http:    reqwest::Client::new(),
            api_key: env::var("VONIX_API_KEY").expect("VONIX_API_KEY not set"), // vx_live_...
        }
    }

    async fn get(&self, path: &str) -> serde_json::Value {
        self.http
            .get(format!("{BASE_URL}{path}"))
            .bearer_auth(&self.api_key)
            .send().await.unwrap()
            .json().await.unwrap()
    }

    async fn post(&self, path: &str, body: serde_json::Value) -> serde_json::Value {
        self.http
            .post(format!("{BASE_URL}{path}"))
            .bearer_auth(&self.api_key)
            .json(&body)
            .send().await.unwrap()
            .json().await.unwrap()
    }

    // 1. Get all balances
    pub async fn get_balance(&self) -> serde_json::Value {
        self.get("/v1/balance").await
    }

    // 2. Send a fiat payment
    pub async fn send_fiat_payment(
        &self, currency: &str, amount: &str, recipient_name: &str, iban: &str, reference: &str,
    ) -> serde_json::Value {
        let ts = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_secs();
        self.post("/v1/transfers", serde_json::json!({
            "currency":    currency,
            "amount":      amount,
            "recipient":   { "name": recipient_name, "iban": iban },
            "reference":   reference,
            "external_id": format!("order_{ts}"),
        })).await
    }

    // 3. Get a crypto deposit address
    pub async fn get_deposit_address(&self, currency: &str, network: &str) -> serde_json::Value {
        self.get(&format!("/v1/crypto/deposit-address?currency={currency}&network={network}")).await
    }

    // 4. Send crypto
    pub async fn send_crypto(
        &self, currency: &str, network: &str, amount: &str, address: &str,
    ) -> serde_json::Value {
        self.post("/v1/crypto/send", serde_json::json!({
            "currency": currency,
            "network":  network,
            "amount":   amount,
            "address":  address,
        })).await
    }

    // 5. Register a webhook
    pub async fn register_webhook(&self, url: &str) -> serde_json::Value {
        self.post("/v1/webhooks", serde_json::json!({
            "url":    url,
            "events": ["transfer.completed", "transfer.failed", "deposit.confirmed"],
        })).await
    }
}

// 6. Verify an incoming webhook signature
fn verify_webhook_signature(secret: &str, timestamp: &str, signature: &str, raw_body: &str) -> bool {
    let mut mac = Hmac::<Sha256>::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(format!("{timestamp}.{raw_body}").as_bytes());
    let expected = format!("sha256={}", hex::encode(mac.finalize().into_bytes()));
    expected == signature
}

#[tokio::main]
async fn main() {
    let client = VonixClient::new();

    println!("\n── Balance ─────────────────────────────");
    println!("{}", serde_json::to_string_pretty(&client.get_balance().await).unwrap());

    println!("\n── Send Fiat ────────────────────────────");
    println!("{}", serde_json::to_string_pretty(
        &client.send_fiat_payment("USD", "250.00", "ACME Corp", "GB29NWBK60161331926819", "INV-2024-042").await
    ).unwrap());

    println!("\n── Deposit Address (USDT/Ethereum) ──────");
    println!("{}", serde_json::to_string_pretty(
        &client.get_deposit_address("USDT", "ethereum").await
    ).unwrap());

    println!("\n── Send Crypto ──────────────────────────");
    println!("{}", serde_json::to_string_pretty(
        &client.send_crypto("USDT", "ethereum", "100.000000", "0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt").await
    ).unwrap());
}
