# Discord Webhook Setup

Step-by-step guide to create a Discord webhook for bambu-notifier.

## Create a Webhook

1. Open Discord and navigate to the server where you want notifications.
2. Click the server name at the top-left and select **Server Settings**.
3. In the left sidebar, select **Integrations**.
4. Click **Webhooks**, then **New Webhook**.
5. Give the webhook a name (e.g., "Bambu Notifier").
6. Select the channel where notifications should be posted.
7. Click **Copy Webhook URL**.

<!-- Screenshot: Discord Server Settings > Integrations > Webhooks panel -->

## Configure bambu-notifier

Paste the webhook URL into your `config.toml`:

```toml
[[printer.notifier]]
type        = "discord"
webhook_url = "https://discord.com/api/webhooks/1234567890/abcdef..."
secret      = ""
```

## Optional: HMAC Signing

bambu-notifier supports signing webhook payloads with HMAC-SHA256. When a `secret` is set, every request includes an `X-Hub-Signature-256` header containing the signature.

This is useful if you route webhooks through a proxy or middleware that validates signatures before forwarding to Discord.

```toml
[[printer.notifier]]
type        = "discord"
webhook_url = "https://your-proxy.example.com/webhook"
secret      = "your-shared-secret"
```

The signature format is `sha256=<hex-encoded-hmac>`.
