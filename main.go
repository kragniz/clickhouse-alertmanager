package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/kragniz/clickhouse-alertmanager/alert"
	"github.com/kragniz/clickhouse-alertmanager/config"
	"github.com/kragniz/clickhouse-alertmanager/rule"
)

func Fatal(err error) {
	slog.Error("Fatal", "error", err)
	os.Exit(1)
}

func connect() (driver.Conn, error) {
	ctx := context.Background()
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"sql-clickhouse.clickhouse.com:9440"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "demo",
			Password: "",
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

		TLS: &tls.Config{},

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
	logger := slog.New(
		slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: slog.LevelDebug,
			},
		),
	)

	slog.SetDefault(logger)

	conn, err := connect()
	if err != nil {
		Fatal(err)
	}

	config, err := config.ReadAlertConfig("alerts.yaml")
	if err != nil {
		Fatal(err)
	}

	scheduledRules := rule.ScheduledRulesFromConfig(*config, conn)

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
						alert.Send(alerts)
					}

					if err != nil {
						slog.Error("AlertForConfig", "error", err)
					}
				}
			}
		}

		time.Sleep(1 * time.Second)
	}
}
