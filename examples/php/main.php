<?php
/**
 * Vonix Business API — PHP examples
 * Requires: PHP 7.4+ with cURL extension
 */

define('BASE_URL', 'https://api.getvonix.com');
define('API_KEY',  getenv('VONIX_API_KEY')); // vx_live_...

function vonixRequest(string $method, string $path, array $body = []): array
{
    $ch = curl_init(BASE_URL . $path);
    $headers = [
        'Authorization: Bearer ' . API_KEY,
        'Content-Type: application/json',
    ];
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
    if ($method === 'POST') {
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($body));
    } elseif ($method === 'DELETE') {
        curl_setopt($ch, CURLOPT_CUSTOMREQUEST, 'DELETE');
    }
    $response = curl_exec($ch);
    curl_close($ch);
    return json_decode($response, true);
}

// 1. Get all balances
function getBalance(): array
{
    return vonixRequest('GET', '/v1/balance');
}

// 2. Send a fiat payment (SEPA/SWIFT)
function sendFiatPayment(
    string $currency,
    string $amount,
    string $recipientName,
    string $iban,
    string $reference
): array {
    return vonixRequest('POST', '/v1/transfers', [
        'currency'    => $currency,
        'amount'      => $amount,
        'recipient'   => ['name' => $recipientName, 'iban' => $iban],
        'reference'   => $reference,
        'external_id' => 'order_' . time(),
    ]);
}

// 3. Get a crypto deposit address
function getDepositAddress(string $currency = 'USDT', string $network = 'ethereum'): array
{
    return vonixRequest('GET', '/v1/crypto/deposit-address?currency=' . $currency . '&network=' . $network);
}

// 4. Send crypto
function sendCrypto(string $currency, string $network, string $amount, string $address): array
{
    return vonixRequest('POST', '/v1/crypto/send', [
        'currency' => $currency,
        'network'  => $network,
        'amount'   => $amount,
        'address'  => $address,
    ]);
}

// 5. Register a webhook
function registerWebhook(string $url): array
{
    return vonixRequest('POST', '/v1/webhooks', [
        'url'    => $url,
        'events' => ['transfer.completed', 'transfer.failed', 'deposit.confirmed'],
    ]);
}

// 6. Verify an incoming webhook signature
function verifyWebhookSignature(string $secret, string $timestamp, string $signature, string $rawBody): bool
{
    $expected = 'sha256=' . hash_hmac('sha256', $timestamp . '.' . $rawBody, $secret);
    return hash_equals($expected, $signature);
}

// ── Run examples ─────────────────────────────
echo "\n── Balance ─────────────────────────────\n";
print_r(getBalance());

echo "\n── Send Fiat ────────────────────────────\n";
print_r(sendFiatPayment('USD', '250.00', 'ACME Corp', 'GB29NWBK60161331926819', 'INV-2024-042'));

echo "\n── Deposit Address (USDT/Ethereum) ──────\n";
print_r(getDepositAddress('USDT', 'ethereum'));

echo "\n── Send Crypto ──────────────────────────\n";
print_r(sendCrypto('USDT', 'ethereum', '100.000000', '0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt'));
