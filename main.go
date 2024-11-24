package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/kragniz/clickhouse-alertmanager/alert"
	"github.com/kragniz/clickhouse-alertmanager/config"
	"github.com/kragniz/clickhouse-alertmanager/metrics"
	"github.com/kragniz/clickhouse-alertmanager/rule"
)

func Fatal(err error) {
	slog.Error("Fatal", "error", err)
	os.Exit(1)
}

func connect(conf config.Clickhouse) (driver.Conn, error) {
	ctx := context.Background()

	var tlsConf *tls.Config
	if conf.TLS {
		tlsConf = &tls.Config{}
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: conf.Addresses,
		Auth: clickhouse.Auth{
			Database: conf.Database,
			Username: conf.Username,
			Password: conf.Password,
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct {
				Name    string
				Version string
			}{
				{
					Name:    "clickhouse-alertmanager",
					Version: "0.1.0",
				},
			},
		},

		TLS: tlsConf,

		Debug: false,

		Debugf: func(format string, v ...any) {
			slog.Info("clickhouse-go", "msg", fmt.Sprintf(format, v...))
		},
	})

	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			slog.Error("Exception",
				"code", exception.Code,
				"msg", exception.Message,
				"stacktrace", exception.StackTrace,
			)
		}
		return nil, err
	}
	return conn, nil
}

func main() {
	configFile := flag.String("config.file", "config.yaml", "Config file path")
	logLevel := flag.String("log.level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		Fatal(fmt.Errorf("invalid log level: %s", *logLevel))
	}

	logger := slog.New(
		slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: level,
			},
		),
	)
	slog.SetDefault(logger)

	conf, err := config.ReadConfig(*configFile)
	if err != nil {
		Fatal(err)
	}

	go metrics.ListenAndServe()

	conn, err := connect(conf.Clickhouse)
	if err != nil {
		Fatal(err)
	}

	scheduledRules := []rule.ScheduledRule{}
	for _, alertConfigFile := range conf.RuleFiles {
		alertconfig, err := config.ReadAlertConfig(alertConfigFile)
		if err != nil {
			Fatal(err)
		}

		scheduledRules = slices.Concat(scheduledRules, rule.ScheduledRulesFromConfig(*alertconfig, conn))
	}

	if len(scheduledRules) == 0 {
		Fatal(fmt.Errorf("no rules found"))
	}

	alertmanagers := alert.AlertmanagersFromConfig(*conf)

	for {
		for i, scheduledRule := range scheduledRules {
			if !scheduledRule.Running {
				if time.Since(scheduledRule.LastRun) > 5*time.Second {
					slog.Info("Running rule",
						"group", scheduledRule.GroupName,
						"rule", scheduledRule.Config.AlertName,
					)

					alerts, err := scheduledRules[i].Run()
					if err != nil {
						slog.Info("Error running rule",
							"group", scheduledRule.GroupName,
							"rule", scheduledRule.Config.AlertName,
							"error", err,
						)
					}

					if len(alerts) > 0 {
						alertmanagers.Send(alerts)
					}

					if err != nil {
						slog.Error("AlertForConfig", "error", err)
					}
				}
			}
		}

		time.Sleep(time.Duration(conf.EvaluationInterval) * time.Second)
	}
}
