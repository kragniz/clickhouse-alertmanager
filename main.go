package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"reflect"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/kragniz/clickhouse-alertmanager/alert"
	"github.com/kragniz/clickhouse-alertmanager/config"
)

type ScheduledRule struct {
	LastRun   time.Time
	Running   bool
	Config    config.Rule
	GroupName string
	Labels    map[string]string
}

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

		TLS: &tls.Config{InsecureSkipVerify: true},

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

func query(conn driver.Conn, query string) ([]map[string]string, error) {
	ctx := context.Background()
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var objects []map[string]any

	for rows.Next() {
		// FIXME: try and do this reflection stuff outside the loop

		columns := rows.ColumnTypes()

		values := make([]any, len(columns))
		object := map[string]any{}

		for i, column := range columns {
			object[column.Name()] = reflect.New(column.ScanType()).Interface()
			values[i] = object[column.Name()]
		}

		if err = rows.Scan(values...); err != nil {
			slog.Error("Scanning rows", "error", err)
			return nil, err
		}

		objects = append(objects, object)
	}

	for _, v := range objects {
		values := []slog.Attr{}
		for k, v := range v {
			concreteValue := reflect.ValueOf(v).Elem()
			values = append(values, slog.Any(k, concreteValue))
		}
		slog.LogAttrs(
			context.Background(),
			slog.LevelDebug,
			"Rule found",
			values...,
		)
	}

	labels := []map[string]string{}
	for _, v := range objects {
		alertLabels := map[string]string{}
		for k, v := range v {
			var value string
			// TODO: support more types
			switch v := reflect.ValueOf(v).Elem().Interface().(type) {
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				value = fmt.Sprintf("%d", v)
			case float32, float64:
				value = fmt.Sprintf("%f", v)
			case string:
				value = v
			default:
				slog.Warn("Unsupported type", "type", reflect.TypeOf(v))
				value = fmt.Sprintf("%+v", v)
			}

			alertLabels[k] = value
		}
		labels = append(labels, alertLabels)
	}

	return labels, nil
}

func ScheduledRulesFromConfig(c config.AlertConfig) []ScheduledRule {
	rules := []ScheduledRule{}
	for _, group := range c.Groups {
		for _, rule := range group.Rules {
			labels := maps.Clone(group.Labels)
			maps.Copy(labels, rule.Labels)
			rules = append(rules, ScheduledRule{
				LastRun:   time.Time{},
				Running:   false,
				Config:    rule,
				Labels:    labels,
				GroupName: group.Name,
			})
		}
	}
	return rules
}

func RunRule(rule *ScheduledRule, conn driver.Conn) ([]alert.ActiveAlert, error) {
	alerts := []alert.ActiveAlert{}

	rule.Running = true

	queryResult, err := query(conn, rule.Config.Expr)
	if err != nil {
		rule.Running = false
		rule.LastRun = time.Now()
		return nil, err
	}

	for _, queryLabels := range queryResult {
		labels := maps.Clone(rule.Labels)
		maps.Copy(labels, queryLabels)
		labels["alertname"] = rule.Config.AlertName

		alerts = append(alerts, alert.ActiveAlert{
			Labels:      labels,
			Annotations: rule.Config.Annotations,
		})
	}

	rule.Running = false
	rule.LastRun = time.Now()

	return alerts, nil
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
		panic((err))
	}

	config, err := config.ReadAlertConfig("alerts.yaml")
	if err != nil {
		panic((err))
	}

	scheduledRules := ScheduledRulesFromConfig(*config)

	for {
		for i, scheduledRule := range scheduledRules {
			if !scheduledRule.Running {
				if time.Since(scheduledRule.LastRun) > 5*time.Second {
					slog.Info("Running rule",
						"group", scheduledRule.GroupName,
						"rule", scheduledRule.Config.AlertName,
					)

					alerts, err := RunRule(&scheduledRules[i], conn)
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
