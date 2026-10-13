// Vonix Business API — C# examples
// Requires: .NET 6+ (uses HttpClient and System.Text.Json)

using System.Net.Http.Headers;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

var baseUrl = "https://api.getvonix.com";
var apiKey  = Environment.GetEnvironmentVariable("VONIX_API_KEY")!; // vx_live_...

using var http = new HttpClient();
http.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", apiKey);
http.DefaultRequestHeaders.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));

// ── Helper ────────────────────────────────────
async Task<JsonDocument> Request(HttpMethod method, string path, object? body = null)
{
    var req = new HttpRequestMessage(method, baseUrl + path);
    if (body is not null)
        req.Content = new StringContent(JsonSerializer.Serialize(body), Encoding.UTF8, "application/json");
    var resp = await http.SendAsync(req);
    return JsonDocument.Parse(await resp.Content.ReadAsStringAsync());
}

// 1. Get all balances
async Task<JsonDocument> GetBalance() =>
    await Request(HttpMethod.Get, "/v1/balance");

// 2. Send a fiat payment
async Task<JsonDocument> SendFiatPayment(
    string currency, string amount, string recipientName, string iban, string reference) =>
    await Request(HttpMethod.Post, "/v1/transfers", new {
        currency,
        amount,
        recipient   = new { name = recipientName, iban },
        reference,
        external_id = $"order_{DateTimeOffset.UtcNow.ToUnixTimeSeconds()}",
    });

// 3. Get a crypto deposit address
async Task<JsonDocument> GetDepositAddress(string currency = "USDT", string network = "ethereum") =>
    await Request(HttpMethod.Get, $"/v1/crypto/deposit-address?currency={currency}&network={network}");

// 4. Send crypto
async Task<JsonDocument> SendCrypto(string currency, string network, string amount, string address) =>
    await Request(HttpMethod.Post, "/v1/crypto/send", new { currency, network, amount, address });

// 5. Register a webhook
async Task<JsonDocument> RegisterWebhook(string url) =>
    await Request(HttpMethod.Post, "/v1/webhooks", new {
        url,
        events = new[] { "transfer.completed", "transfer.failed", "deposit.confirmed" },
    });

// 6. Verify an incoming webhook signature
bool VerifyWebhookSignature(string secret, string timestamp, string signature, string rawBody)
{
    using var mac = new HMACSHA256(Encoding.UTF8.GetBytes(secret));
    var hash     = mac.ComputeHash(Encoding.UTF8.GetBytes($"{timestamp}.{rawBody}"));
    var expected = "sha256=" + Convert.ToHexString(hash).ToLowerInvariant();
    return CryptographicOperations.FixedTimeEquals(
        Encoding.UTF8.GetBytes(expected), Encoding.UTF8.GetBytes(signature));
}

// ── Run examples ─────────────────────────────
Console.WriteLine("\n── Balance ─────────────────────────────");
Console.WriteLine((await GetBalance()).RootElement);

Console.WriteLine("\n── Send Fiat ────────────────────────────");
Console.WriteLine((await SendFiatPayment("USD", "250.00", "ACME Corp", "GB29NWBK60161331926819", "INV-2024-042")).RootElement);

Console.WriteLine("\n── Deposit Address (USDT/Ethereum) ──────");
Console.WriteLine((await GetDepositAddress("USDT", "ethereum")).RootElement);

Console.WriteLine("\n── Send Crypto ──────────────────────────");
Console.WriteLine((await SendCrypto("USDT", "ethereum", "100.000000", "0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt")).RootElement);
