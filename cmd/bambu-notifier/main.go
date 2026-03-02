package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/l33t0/bambu-notifier/internal/config"
	"github.com/l33t0/bambu-notifier/internal/notifier"
	"github.com/l33t0/bambu-notifier/internal/printer"
)

var configPath string

var rootCmd = &cobra.Command{
	Use:   "bambu-notifier",
	Short: "Monitors Bambu Lab printers via MQTT and sends notifications",
	RunE:  run,
}

func init() {
	rootCmd.Flags().StringVar(
		&configPath, "config", "config.toml",
		"path to the TOML configuration file",
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := setupLogger(cfg.Log)
	slog.SetDefault(logger)

	logger.Info("configuration loaded",
		slog.Int("printers", len(cfg.Printers)),
	)

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	printers := buildPrinters(cfg, logger)

	g, gctx := errgroup.WithContext(ctx)
	for _, p := range printers {
		p := p
		g.Go(func() error {
			return p.Start(gctx)
		})
	}

	logger.Info("bambu-notifier started",
		slog.Int("printers", len(printers)),
	)

	<-ctx.Done()
	logger.Info("shutting down")

	for _, p := range printers {
		p.Stop()
	}

	return g.Wait()
}

func buildPrinters(
	cfg config.Config,
	logger *slog.Logger,
) []printer.Printer {
	printers := make([]printer.Printer, 0, len(cfg.Printers))

	for _, pc := range cfg.Printers {
		notifiers := buildNotifiers(pc)

		p := printer.New(printer.PrinterConfig{
			Name:                  pc.Name,
			Host:                  pc.Host,
			Port:                  pc.Port,
			SerialNumber:          pc.SerialNumber,
			AccessCode:            pc.AccessCode,
			TLSInsecureSkipVerify: pc.TLSInsecureSkipVerify,
			ReconnectDelaySeconds: pc.ReconnectDelaySeconds,
		}, notifiers, logger)

		printers = append(printers, p)
	}

	return printers
}

func buildNotifiers(pc config.Printer) []notifier.Notifier {
	var notifiers []notifier.Notifier

	for _, d := range pc.Notifiers.Discord {
		notifiers = append(notifiers,
			notifier.NewDiscord(d.Name, d.WebhookURL, d.Secret),
		)
	}

	for _, s := range pc.Notifiers.Slack {
		notifiers = append(notifiers,
			notifier.NewSlack(s.Name, s.WebhookURL, s.Secret),
		)
	}

	for _, t := range pc.Notifiers.Telegram {
		notifiers = append(notifiers,
			notifier.NewTelegram(t.Name, t.BotToken, t.ChatID),
		)
	}

	return notifiers
}

func setupLogger(l config.Log) *slog.Logger {
	level := slog.LevelInfo

	switch l.Level {
	case config.LogLevelDebug:
		level = slog.LevelDebug
	case config.LogLevelWarn:
		level = slog.LevelWarn
	case config.LogLevelError:
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch l.Format {
	case config.LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
