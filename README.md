# GitHub Stats Exporter

![Go Version](https://img.shields.io/badge/go-1.25-blue)
![Docker](https://img.shields.io/badge/docker-ready-blue)
![License](https://img.shields.io/badge/license-MIT-green)

A **generic, configuration-driven** Prometheus exporter for GitHub.

Unlike standard exporters that hardcode specific metrics (e.g., just "stars" or "forks"), this exporter acts as a **bridge between the GitHub API and Prometheus**. You define *what* you want to measure in YAML using JSON paths and GraphQL queries, and the exporter handles the authentication, scraping, and metric exposure.

---

## 🚀 Features

* **Configurable Metrics:** Define new metrics in `config.yaml` without changing Go code.
* **Powerful Parsing:** Uses [GJSON](https://github.com/tidwall/gjson) syntax for complex JSON extraction and aggregation.
* **GraphQL Support:** Native support for GitHub GraphQL API (e.g., retrieving "Contribution Calendar" squares).
* **Golang Templating:** Supports `{{ .GITHUB_USER }}` interpolation in configuration files via CLI flags or Environment Variables.
* **Secure:** secrets managed via `GITHUB_TOKEN` environment variable.
* **Lightweight:** Built on Alpine, <20MB Docker image.

---

## 🛠 Usage

### 1. Run Locally

```bash
export GITHUB_TOKEN="ghp_your_token_here"
export GITHUB_USER="your_github_username"
# 2. Run with a username override
go run main.go --config config.yaml
```

## ⚙️ Configuration (config.yaml)
The configuration uses Go templates. You can use {{ .GITHUB_USER }} anywhere in the file, and it will be replaced at runtime by the value provided in the --github-user flag or GITHUB_USER env var.

### REST API Example (Search)
Fetches total merged PRs for the user.
```YAML
requests:
  - api_path: "/search/issues?q=author:{{ .GITHUB_USER }}+type:pr+is:merged"
    metrics:
      - name: gh_prs_merged_total
        path: "total_count"
        help: "Total Pull Requests merged by {{ .GITHUB_USER }}"
```
### Aggregation Example
Fetches all repos and sums up the stars.

```YAML
requests:
  - api_path: "/users/{{ .GITHUB_USER }}/repos?per_page=100"
    metrics:
      - name: gh_stars_total
        path: "#.stargazers_count" # GJSON: Get all stargazer counts
        aggregate: "sum"           # Options: sum, count, max
        help: "Total stars across all repositories"
### GraphQL Example
Fetches the "Green Squares" (Contribution Calendar).

```YAML
requests:
  - api_path: "/graphql"
    method: "POST"
    # Note: JSON inside YAML requires careful escaping
    body: |
      { "query": "query { user(login: \"{{ .GITHUB_USER }}\") { contributionsCollection { contributionCalendar { totalContributions } } } }" }
    metrics:
      - name: gh_contributions_last_year
        path: "data.user.contributionsCollection.contributionCalendar.totalContributions"
        help: "Total contributions in the last year"
```

## Metrics

Metrics are exposed on :2112/metrics.

Add the following service monitor to the deployment to scrape metrics with Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: github-exporter
  namespace: monitoring
  labels:
    release: prometheus # Must match your Prometheus Operator selector
spec:
  selector:
    matchLabels:
      app: github-exporter
  endpoints:
    - port: metrics # This assumes you named the port 'metrics' in your Service
      interval: 30m
      path: /metrics
```
