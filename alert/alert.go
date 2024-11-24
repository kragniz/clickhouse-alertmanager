package alert

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/kragniz/clickhouse-alertmanager/config"
	"github.com/kragniz/clickhouse-alertmanager/metrics"
)

type Alertmanager struct {
	Endpoint string
}

type Alertmanagers struct {
	Alertmanagers []Alertmanager
}

type ActiveAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func AlertmanagersFromConfig(conf config.Config) Alertmanagers {
	alertmanagers := []Alertmanager{}
	for _, alertmanager := range conf.Alertmanager.StaticConfig.Targets {
		url := url.URL{
			Scheme: conf.Alertmanager.Scheme,
			Host:   alertmanager,
			Path:   "/api/v2/alerts",
		}
		alertmanagers = append(alertmanagers, Alertmanager{Endpoint: url.String()})
	}
	return Alertmanagers{Alertmanagers: alertmanagers}
}

func (a Alertmanager) Send(alerts []ActiveAlert) error {
	request, err := json.Marshal(alerts)
	if err != nil {
		return err
	}

	resp, err := http.Post(a.Endpoint, "application/json", bytes.NewBuffer(request))
	if err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Info("Alert sent", "endpoint", a.Endpoint, "status", resp.Status, "body", string(body))

	metrics.AlertsSent.Add(float64(len(alerts)))

	return nil
}

func (a Alertmanagers) Send(alerts []ActiveAlert) {
	for _, alertmanager := range a.Alertmanagers {
		err := alertmanager.Send(alerts)
		if err != nil {
			slog.Error("Failed to send alerts to alertmanager", "error", err)
		}
	}
}
