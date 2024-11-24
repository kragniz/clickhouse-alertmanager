package alert

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type ActiveAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func Send(alerts []ActiveAlert) error {
	alertsEndpoint := "http://localhost:9093/api/v2/alerts"

	request, err := json.Marshal(alerts)
	if err != nil {
		return err
	}

	resp, err := http.Post(alertsEndpoint, "application/json", bytes.NewBuffer(request))
	if err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Info("Alert sent", "status", resp.Status, "body", string(body))
	return nil
}
