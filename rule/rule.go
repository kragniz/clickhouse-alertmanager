package rule

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"regexp"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/kragniz/clickhouse-alertmanager/alert"
	"github.com/kragniz/clickhouse-alertmanager/config"
	"github.com/kragniz/clickhouse-alertmanager/metrics"
)

type ScheduledRule struct {
	LastRun   time.Time
	Running   bool
	Config    config.Rule
	GroupName string
	Labels    map[string]string

	conn driver.Conn
}

var nonPrometheusLabelRegex = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func ScheduledRulesFromConfig(c config.AlertConfig, conn driver.Conn) []ScheduledRule {
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

				conn: conn,
			})
		}
	}

	metrics.RulesActive.Set(float64(len(rules)))

	return rules
}

func (rule *ScheduledRule) Query() ([]map[string]string, error) {
	ctx := context.Background()

	start := time.Now()

	rows, err := rule.conn.Query(ctx, rule.Config.Expr)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	duration := time.Since(start).Seconds()
	metrics.QueryDuration.Observe(duration)

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

			k = nonPrometheusLabelRegex.ReplaceAllString(k, "_")

			alertLabels[k] = value
		}
		labels = append(labels, alertLabels)
		slog.Debug("Query found", "labels", alertLabels)
	}

	return labels, nil
}

func (rule *ScheduledRule) Run() ([]alert.ActiveAlert, error) {
	alerts := []alert.ActiveAlert{}

	rule.Running = true

	queryResult, err := rule.Query()
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

	metrics.RulesProcessed.WithLabelValues(rule.GroupName, rule.Config.AlertName).Inc()

	return alerts, nil
}
