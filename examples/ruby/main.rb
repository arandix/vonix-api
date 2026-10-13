# Vonix Business API — Ruby examples
# Requires: Ruby 2.7+ (no extra gems needed)

require 'net/http'
require 'json'
require 'openssl'
require 'uri'

BASE_URL = 'https://api.getvonix.com'
API_KEY  = ENV['VONIX_API_KEY'] # vx_live_...

def vonix_request(method, path, body = nil)
  uri  = URI("#{BASE_URL}#{path}")
  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true

  req_class = { 'GET' => Net::HTTP::Get, 'POST' => Net::HTTP::Post, 'DELETE' => Net::HTTP::Delete }[method]
  req = req_class.new(uri)
  req['Authorization'] = "Bearer #{API_KEY}"
  req['Content-Type']  = 'application/json'
  req.body = body.to_json if body

  response = http.request(req)
  JSON.parse(response.body)
end

# 1. Get all balances
def get_balance
  vonix_request('GET', '/v1/balance')
end

# 2. Send a fiat payment
def send_fiat_payment(currency:, amount:, recipient_name:, iban:, reference:)
  vonix_request('POST', '/v1/transfers', {
    currency:    currency,
    amount:      amount,
    recipient:   { name: recipient_name, iban: iban },
    reference:   reference,
    external_id: "order_#{Time.now.to_i}",
  })
end

# 3. Get a crypto deposit address
def get_deposit_address(currency = 'USDT', network = 'ethereum')
  vonix_request('GET', "/v1/crypto/deposit-address?currency=#{currency}&network=#{network}")
end

# 4. Send crypto
def send_crypto(currency:, network:, amount:, address:)
  vonix_request('POST', '/v1/crypto/send', { currency:, network:, amount:, address: })
end

# 5. Register a webhook
def register_webhook(url)
  vonix_request('POST', '/v1/webhooks', {
    url:    url,
    events: ['transfer.completed', 'transfer.failed', 'deposit.confirmed'],
  })
end

# 6. Verify an incoming webhook signature
def verify_webhook_signature(secret:, timestamp:, signature:, raw_body:)
  expected = 'sha256=' + OpenSSL::HMAC.hexdigest('SHA256', secret, "#{timestamp}.#{raw_body}")
  OpenSSL.fixed_length_secure_compare(expected, signature)
end

# ── Run examples ─────────────────────────────
puts "\n── Balance ─────────────────────────────"
pp get_balance

puts "\n── Send Fiat ────────────────────────────"
pp send_fiat_payment(currency: 'USD', amount: '250.00', recipient_name: 'ACME Corp',
                     iban: 'GB29NWBK60161331926819', reference: 'INV-2024-042')

puts "\n── Deposit Address (USDT/Ethereum) ──────"
pp get_deposit_address('USDT', 'ethereum')

puts "\n── Send Crypto ──────────────────────────"
pp send_crypto(currency: 'USDT', network: 'ethereum', amount: '100.000000',
               address: '0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt')
