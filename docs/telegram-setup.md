# Telegram Bot Setup

Step-by-step guide to create a Telegram bot for bambu-notifier.

## Create a Bot

1. Open Telegram and search for **@BotFather**.
2. Send `/newbot` and follow the prompts to choose a name and username.
3. BotFather will respond with a **bot token** (e.g., `123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11`). Save this.

## Get Your Chat ID

You need the numeric chat ID for the channel or group where notifications should be posted.

**For a group or channel:**

1. Add your bot to the group/channel.
2. Send a message in the group.
3. Open `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates` in a browser.
4. Find the `"chat":{"id":...}` value in the response. Group/channel IDs are negative numbers (e.g., `-100123456789`).

**For direct messages:**

1. Send any message to your bot.
2. Open the same `getUpdates` URL above.
3. Your chat ID is the positive number in `"chat":{"id":...}`.

## Configure bambu-notifier

Add the bot token and chat ID to your `config.toml` under the printer that should send to this chat:

```toml
[[printer.notifiers.telegram]]
name      = "home-alerts"
bot_token = "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
chat_id   = "-100123456789"
```

Messages are sent as HTML-formatted text via the Telegram Bot API `sendMessage` endpoint.
