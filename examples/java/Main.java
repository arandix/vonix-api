/**
 * Vonix Business API — Java examples
 * Requires: Java 11+ (uses java.net.http.HttpClient)
 */

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.util.HexFormat;

public class Main {

    private static final String BASE_URL = "https://api.getvonix.com";
    private static final String API_KEY  = System.getenv("VONIX_API_KEY"); // vx_live_...
    private static final HttpClient HTTP  = HttpClient.newHttpClient();

    static String request(String method, String path, String bodyJson) throws Exception {
        var builder = HttpRequest.newBuilder()
            .uri(URI.create(BASE_URL + path))
            .header("Authorization", "Bearer " + API_KEY)
            .header("Content-Type", "application/json");

        if ("POST".equals(method)) {
            builder.POST(HttpRequest.BodyPublishers.ofString(bodyJson));
        } else if ("DELETE".equals(method)) {
            builder.DELETE();
        } else {
            builder.GET();
        }

        var response = HTTP.send(builder.build(), HttpResponse.BodyHandlers.ofString());
        return response.body();
    }

    // 1. Get all balances
    static String getBalance() throws Exception {
        return request("GET", "/v1/balance", null);
    }

    // 2. Send a fiat payment
    static String sendFiatPayment(
        String currency, String amount, String recipientName, String iban, String reference
    ) throws Exception {
        String body = """
            {
                "currency": "%s",
                "amount": "%s",
                "recipient": { "name": "%s", "iban": "%s" },
                "reference": "%s",
                "external_id": "order_%d"
            }
            """.formatted(currency, amount, recipientName, iban, reference, Instant.now().getEpochSecond());
        return request("POST", "/v1/transfers", body);
    }

    // 3. Get a crypto deposit address
    static String getDepositAddress(String currency, String network) throws Exception {
        return request("GET", "/v1/crypto/deposit-address?currency=" + currency + "&network=" + network, null);
    }

    // 4. Send crypto
    static String sendCrypto(String currency, String network, String amount, String address) throws Exception {
        String body = """
            { "currency": "%s", "network": "%s", "amount": "%s", "address": "%s" }
            """.formatted(currency, network, amount, address);
        return request("POST", "/v1/crypto/send", body);
    }

    // 5. Register a webhook
    static String registerWebhook(String url) throws Exception {
        String body = """
            {
                "url": "%s",
                "events": ["transfer.completed", "transfer.failed", "deposit.confirmed"]
            }
            """.formatted(url);
        return request("POST", "/v1/webhooks", body);
    }

    // 6. Verify an incoming webhook signature
    static boolean verifyWebhookSignature(
        String secret, String timestamp, String signature, String rawBody
    ) throws Exception {
        var mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
        byte[] digest = mac.doFinal((timestamp + "." + rawBody).getBytes(StandardCharsets.UTF_8));
        String expected = "sha256=" + HexFormat.of().formatHex(digest);
        return expected.equals(signature);
    }

    public static void main(String[] args) throws Exception {
        System.out.println("\n── Balance ─────────────────────────────");
        System.out.println(getBalance());

        System.out.println("\n── Send Fiat ────────────────────────────");
        System.out.println(sendFiatPayment("USD", "250.00", "ACME Corp", "GB29NWBK60161331926819", "INV-2024-042"));

        System.out.println("\n── Deposit Address (USDT/Ethereum) ──────");
        System.out.println(getDepositAddress("USDT", "ethereum"));

        System.out.println("\n── Send Crypto ──────────────────────────");
        System.out.println(sendCrypto("USDT", "ethereum", "100.000000", "0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt"));
    }
}
