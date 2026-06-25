package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
	"dispatch/internal/router"
	"dispatch/internal/version"
)

func main() {
	configPath := flag.String("config", "", "path to router.yaml")
	checkConfig := flag.Bool("check-config", false, "validate config and exit (0=ok, 1=error)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("dispatch version %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildTime)
		return
	}

	var cfgPath string
	if *configPath != "" {
		cfgPath = *configPath
	} else {
		cfgPath = filepath.Join("/config", config.DefaultConfigFilename)
	}

	if !*checkConfig {
		dir := filepath.Dir(cfgPath)
		if err := config.EnsureConfigDir(dir); err != nil {
			fmt.Fprintln(os.Stderr, "dispatch:", err)
			os.Exit(1)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dispatch: config error: %v\n", err)
		os.Exit(1)
	}

	setupLogging(cfg)

	slog.Info("config loaded", "path", cfgPath, "model_profiles", len(cfg.ModelProfiles), "levels", len(cfg.Levels), "patterns", len(cfg.Patterns))

	if *checkConfig {
		fmt.Printf("Config OK: %s\n", cfgPath)
		fmt.Printf("\n  Model profiles (%d):\n", len(cfg.ModelProfiles))
		for name, mp := range cfg.ModelProfiles {
			prov := providerSummary(mp.Provider)
			fmt.Printf("    %-18s -> %s%s\n", name, mp.Id, prov)
		}
		fmt.Printf("\n  Routing table:\n")
		for _, name := range []string{"easy", "medium", "hard", "critical"} {
			rm, _ := cfg.ResolveLevel(name)
			prov := providerSummary(rm.Provider)
			source := cfg.Levels[name].Use
			if source == "" {
				source = "(inline)"
			}
			fmt.Printf("    %-10s -> %s  [profile: %s]%s\n", name, rm.Model, source, prov)
		}
		fmt.Printf("\n  patterns: %d rules\n", len(cfg.Patterns))
		fmt.Printf("  thresholds: easy=%d easy_max=%d medium_max=%d hard_max=%d\n",
			int(cfg.Thresholds.Easy), int(cfg.Thresholds.EasyMax),
			int(cfg.Thresholds.MediumMax), int(cfg.Thresholds.HardMax))
		fmt.Printf("  caps: complexity=%.0f risk=%.0f agent_pressure=%.0f downgrade=%.0f\n",
			cfg.Scoring.ComplexityCap, cfg.Scoring.RiskCap,
			cfg.Scoring.AgentPressureCap, cfg.Scoring.DowngradeCap)

		if cfg.OpenRouter.HTTPReferer == "" {
			fmt.Printf("\n  warning: openrouter.http_referer is empty; OpenRouter app attribution may show as Unknown\n")
		}
		if cfg.OpenRouter.SiteTitle == "" {
			fmt.Printf("  warning: openrouter.site_title is empty; OpenRouter app display name will not be set\n")
		}
		return
	}

	apiKey := strings.TrimSpace(os.Getenv(cfg.OpenRouter.APIKeyEnv))
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "dispatch: %s environment variable not set or empty\n", cfg.OpenRouter.APIKeyEnv)
		os.Exit(1)
	}
	keyPrefixValid := strings.HasPrefix(apiKey, "sk-or-")
	if !keyPrefixValid {
		slog.Warn("api key does not have expected OpenRouter prefix (sk-or-); upstream may reject it")
	}

	client := openrouter.NewClient(cfg.OpenRouter.BaseURL, apiKey, cfg.OpenRouter.HTTPReferer, cfg.OpenRouter.SiteTitle)

	rtr := router.New(cfg, client)
	rtr.Stats.SetAPIKeyPresent(true)
	rtr.Stats.SetAPIKeyMeta(keyPrefixValid, len(apiKey))

	stopCh := make(chan struct{})

	if cfg.ConfigReload.Enabled {
		reloader := config.NewReloader(cfgPath, cfg.ConfigReload.PollIntervalSeconds)
		go reloader.Start(func(newCfg *config.Config) {
			rtr.SwapConfig(newCfg)
		}, stopCh)
		slog.Info("config auto-reload enabled", "path", cfgPath, "poll_seconds", cfg.ConfigReload.PollIntervalSeconds)
	}

	mux := http.NewServeMux()
	rtr.RegisterRoutes(mux)

	server := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           mux,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	easyRM, _ := cfg.ResolveLevel("easy")
	mediumRM, _ := cfg.ResolveLevel("medium")
	hardRM, _ := cfg.ResolveLevel("hard")
	criticalRM, _ := cfg.ResolveLevel("critical")
	slog.Info("dispatch ready",
		"easy", easyRM.Model,
		"medium", mediumRM.Model,
		"hard", hardRM.Model,
		"critical", criticalRM.Model,
		"listen", cfg.Server.Listen,
		"openrouter_base", cfg.OpenRouter.BaseURL,
		"api_key_env", cfg.OpenRouter.APIKeyEnv,
		"api_key_present", true,
		"api_key_prefix_valid", keyPrefixValid,
		"api_key_length", len(apiKey),
		"version", version.Version,
	)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func setupLogging(cfg *config.Config) {
	var level slog.Level
	switch cfg.Debug.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func providerSummary(p config.ProviderConfig) string {
	parts := []string{}
	if len(p.Order) > 0 {
		parts = append(parts, "order="+strings.Join(p.Order, ","))
	}
	if p.DataCollection != "" {
		parts = append(parts, "data="+p.DataCollection)
	}
	if p.AllowFallbacks != nil && !*p.AllowFallbacks {
		parts = append(parts, "no_fallback")
	}
	if len(p.Ignore) > 0 {
		parts = append(parts, "ignore="+strings.Join(p.Ignore, ","))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " ") + ")"
}
