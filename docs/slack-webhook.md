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

Paste the webhook URL into your `config.toml`:

```toml
[[printer.notifier]]
type        = "slack"
webhook_url = "https://hooks.slack.com/services/T00000000/B00000000/xxxxxxxxxxxxxxxxxxxxxxxx"
```

## Signing Secret

Slack apps have a **Signing Secret** available under **Basic Information > App Credentials**. This can be used to verify that requests originate from Slack if you build custom integrations on top of bambu-notifier. The signing secret is separate from the webhook URL and is not required for basic webhook delivery.
