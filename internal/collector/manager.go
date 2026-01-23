package collector

import (
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eleboucher/github-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

type MetricInfo struct {
	Desc      *prometheus.Desc
	LabelKeys []string
	Config    config.MetricConfig
}

type Manager struct {
	cfg     *config.Config
	client  *http.Client
	metrics map[string]*MetricInfo
	token   string
}

func NewManager(cfg *config.Config) *Manager {
	// Create transport that disables caching
	transport := &http.Transport{
		DisableKeepAlives: true,
	}

	m := &Manager{
		cfg:     cfg,
		client:  &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		metrics: make(map[string]*MetricInfo),
		token:   cfg.Token,
	}
	m.initDescriptors()
	return m
}

func (m *Manager) initDescriptors() {
	for _, req := range m.cfg.Requests {
		for _, metric := range req.Metrics {
			var labelKeys []string
			labelKeys = append(labelKeys, "api_path")
			for k := range metric.Labels {
				labelKeys = append(labelKeys, k)
			}
			sort.Strings(labelKeys)

			desc := prometheus.NewDesc(
				metric.Name,
				metric.Help,
				labelKeys,
				nil,
			)

			m.metrics[metric.Name] = &MetricInfo{
				Desc:      desc,
				LabelKeys: labelKeys,
				Config:    metric,
			}
		}
	}
}

func (m *Manager) Describe(ch chan<- *prometheus.Desc) {
	for _, info := range m.metrics {
		ch <- info.Desc
	}
}

func (m *Manager) Collect(ch chan<- prometheus.Metric) {
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	for _, req := range m.cfg.Requests {
		wg.Add(1)
		go func(r config.RequestConfig) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			m.fetchAndCollect(r, ch)
		}(req)
	}
	wg.Wait()
}

func (m *Manager) fetchAndCollect(reqCfg config.RequestConfig, ch chan<- prometheus.Metric) {
	path := strings.TrimLeft(reqCfg.ApiPath, "/")
	url := m.cfg.GithubAPIURL + "/" + path

	method := reqCfg.Method
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if reqCfg.Body != "" {
		bodyReader = strings.NewReader(reqCfg.Body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		slog.Error("Error creating request for", "url", url, "err", err)
		return
	}

	req.Header.Set("User-Agent", "eleboucher-github-exporter/1.0")
	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if m.token != "" {
		req.Header.Add("Authorization", "Bearer "+m.token)
	}

	if method == "POST" {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := m.client.Do(req)
	if err != nil {
		slog.Error("Error fetching", "url", url, "err", err)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Error closing response body", "err", err)
		}
	}()

	// Log cache-related headers to debug caching issues
	slog.Debug("Response headers",
		"url", url,
		"etag", resp.Header.Get("ETag"),
		"cache-control", resp.Header.Get("Cache-Control"),
		"age", resp.Header.Get("Age"),
		"x-github-request-id", resp.Header.Get("X-GitHub-Request-Id"))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("Non-200 status code from", "url", url, "status_code", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading response body", "url", url, "err", err)
		return
	}
	jsonStr := string(body)
	for _, metric := range reqCfg.Metrics {
		info, exists := m.metrics[metric.Name]
		if !exists {
			continue
		}

		val := m.parseValue(jsonStr, metric)

		slog.Debug("Parsed metric", "name", metric.Name, "value", val)
		var labelValues []string
		for _, key := range info.LabelKeys {
			if key == "api_path" {
				labelValues = append(labelValues, reqCfg.ApiPath)
				continue
			}
			// Look up the GJSON path for this label
			if jsonPath, ok := metric.Labels[key]; ok {
				res := gjson.Get(jsonStr, jsonPath)
				labelValues = append(labelValues, res.String())
			} else {
				labelValues = append(labelValues, "")
			}
		}

		mType := prometheus.GaugeValue

		m, err := prometheus.NewConstMetric(
			info.Desc,
			mType,
			val,
			labelValues...,
		)
		if err != nil {
			slog.Error("Failed to create metric", "name", metric.Name, "err", err)
			continue
		}

		ch <- m
	}
}

func (m *Manager) parseValue(jsonStr string, metric config.MetricConfig) float64 {
	result := gjson.Get(jsonStr, metric.Path)

	if !result.IsArray() {

		if metric.ValueType == config.TypeDate {
			if result.Type == gjson.String {
				t, err := time.Parse(time.RFC3339, result.String())
				if err != nil {
					slog.Error("Error parsing date for metric", "metric_name", metric.Name, "error", err)
					return 0
				}
				return float64(t.Unix())
			}
			// If it's not a string, we can't parse a date
			return 0
		}
		return result.Float()
	}
	var val float64
	results := result.Array()

	switch metric.Aggregate {
	case config.AggregateCount:
		return float64(len(results))
	case config.AggregateMax:
		if len(results) > 0 {
			val = results[0].Float()
			for _, r := range results[1:] {
				if r.Float() > val {
					val = r.Float()
				}
			}
		}
	case config.AggregateSum: // default
		fallthrough
	default:
		for _, r := range results {
			val += r.Float()
		}
	}
	return val
}
