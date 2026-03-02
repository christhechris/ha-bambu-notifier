# Slack Webhook Setup

Step-by-step guide to create a Slack Incoming Webhook for bambu-notifier.

## Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps).
2. Click **Create New App** and choose **From scratch**.
3. Enter an app name (e.g., "Bambu Notifier") and select your workspace.
4. Click **Create App**.

## Enable Incoming Webhooks

1. In the app settings sidebar, click **Incoming Webhooks**.
2. Toggle **Activate Incoming Webhooks** to **On**.
3. Scroll down and click **Add New Webhook to Workspace**.
4. Select the channel where notifications should appear and click **Allow**.
5. Copy the webhook URL that appears (starts with `https://hooks.slack.com/services/...`).

<!-- Screenshot: Slack App Settings > Incoming Webhooks panel -->

## Configure bambu-notifier

Paste the webhook URL into your `config.toml` under the printer that should send to this channel:

```toml
[[printer.notifiers.slack]]
name        = "ops-channel"
webhook_url = "https://hooks.slack.com/services/T00000000/B00000000/xxxxxxxxxxxxxxxxxxxxxxxx"
secret      = ""   # optional HMAC signing secret
```

## Optional: HMAC Signing

If you route webhooks through a proxy that validates signatures, set a `secret` to enable HMAC-SHA256 signing. Each request will include an `X-Hub-Signature-256` header with the format `sha256=<hex-encoded-hmac>`.
