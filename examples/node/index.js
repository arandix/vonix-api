/**
 * Vonix Business API — Node.js examples
 *
 * npm install node-fetch  (or use native fetch in Node 18+)
 */

const BASE_URL = "https://api.getvonix.com";
const API_KEY  = process.env.VONIX_API_KEY; // vx_live_...

const headers = {
  "Authorization": `Bearer ${API_KEY}`,
  "Content-Type":  "application/json",
};

// ──────────────────────────────────────────────
// 1. Get all balances
// ──────────────────────────────────────────────
async function getBalance() {
  const res = await fetch(`${BASE_URL}/v1/balance`, { headers });
  return res.json();
}

// ──────────────────────────────────────────────
// 2. Send a fiat payment (SEPA/SWIFT)
// ──────────────────────────────────────────────
async function sendFiatPayment({ currency, amount, recipientName, iban, reference }) {
  const res = await fetch(`${BASE_URL}/v1/transfers`, {
    method: "POST",
    headers,
    body: JSON.stringify({
      currency,
      amount,
      recipient: { name: recipientName, iban },
      reference,
      external_id: `order_${Date.now()}`,
    }),
  });
  return res.json();
}

// ──────────────────────────────────────────────
// 3. Get a crypto deposit address
// ──────────────────────────────────────────────
async function getDepositAddress(currency = "USDT", network = "ethereum") {
  const res = await fetch(
    `${BASE_URL}/v1/crypto/deposit-address?currency=${currency}&network=${network}`,
    { headers }
  );
  return res.json();
}

// ──────────────────────────────────────────────
// 4. Send crypto
// ──────────────────────────────────────────────
async function sendCrypto({ currency, network, amount, address }) {
  const res = await fetch(`${BASE_URL}/v1/crypto/send`, {
    method: "POST",
    headers,
    body: JSON.stringify({ currency, network, amount, address }),
  });
  return res.json();
}

// ──────────────────────────────────────────────
// 5. Register a webhook
// ──────────────────────────────────────────────
async function registerWebhook(url) {
  const res = await fetch(`${BASE_URL}/v1/webhooks`, {
    method: "POST",
    headers,
    body: JSON.stringify({
      url,
      events: ["transfer.completed", "transfer.failed", "deposit.confirmed"],
    }),
  });
  const webhook = await res.json();
  console.log("Store this secret securely:", webhook.secret);
  return webhook;
}

// ──────────────────────────────────────────────
// 6. Verify an incoming webhook signature
// ──────────────────────────────────────────────
const crypto = require("crypto");

function verifyWebhookSignature({ secret, timestamp, signature, rawBody }) {
  const expected = "sha256=" + crypto
    .createHmac("sha256", secret)
    .update(`${timestamp}.${rawBody}`)
    .digest("hex");
  return crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(signature));
}

// Express webhook receiver example:
//
// app.post("/webhooks/vonix", express.raw({ type: "application/json" }), (req, res) => {
//   const isValid = verifyWebhookSignature({
//     secret:    process.env.VONIX_WEBHOOK_SECRET,
//     timestamp: req.headers["vonix-timestamp"],
//     signature: req.headers["vonix-signature"],
//     rawBody:   req.body.toString(),
//   });
//   if (!isValid) return res.status(401).send("Invalid signature");
//
//   const event = JSON.parse(req.body);
//   switch (event.event) {
//     case "transfer.completed": handleTransferCompleted(event.data); break;
//     case "deposit.confirmed":  handleDepositConfirmed(event.data);  break;
//   }
//   res.status(200).send("ok");
// });

// ──────────────────────────────────────────────
// Run all examples
// ──────────────────────────────────────────────
(async () => {
  console.log("\n── Balance ─────────────────────────────");
  console.log(JSON.stringify(await getBalance(), null, 2));

  console.log("\n── Send Fiat ────────────────────────────");
  console.log(JSON.stringify(await sendFiatPayment({
    currency:      "USD",
    amount:        "250.00",
    recipientName: "ACME Corp",
    iban:          "GB29NWBK60161331926819",
    reference:     "INV-2024-042",
  }), null, 2));

  console.log("\n── Deposit Address (USDT/Ethereum) ──────");
  console.log(JSON.stringify(await getDepositAddress("USDT", "ethereum"), null, 2));

  console.log("\n── Send Crypto ──────────────────────────");
  console.log(JSON.stringify(await sendCrypto({
    currency: "USDT",
    network:  "ethereum",
    amount:   "100.000000",
    address:  "0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt",
  }), null, 2));
})();
