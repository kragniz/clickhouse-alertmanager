# clickhouse-alertmanager

A small service which acts a bit like prometheus alert rules, but for arbitrary clickhouse queries.

Rules are configured with a file similar to normal prometheus config:

```yaml
groups:
- name: example
  labels:
    team: myteam
  rules:
  - alert: BadNumber
    expr: |
      SELECT
          thing,
          other_thing
      FROM example
      WHERE bad_number > 3
    labels:
      severity: page
    annotations:
      summary: High bad number
```

If the query matches any rows, the results are sent to configured alertmanagers
with selected column values as labels.

Usage:

```
  --config.file string
    	Config file path (default "config.yaml")
  --log.level string
    	Log level (debug, info, warn, error) (default "info")
```

Basic config file:

```yaml
clickhouse:
  addresses:
    - example-host:9440
  tls: true
  database: default
  username: demo
  password: ""

alertmanager:
  scheme: http
  static_config:
    targets:
      - alertmanager-example:9093

rule_files:
  - alerts.yaml

evaluation_interval: 30
```