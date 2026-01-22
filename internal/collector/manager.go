package collector

import (
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/eleboucher/github-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

type Manager struct {
	cfg       *config.Config
	client    *http.Client
	metricMap map[string]*prometheus.GaugeVec
	token     string
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		client:    &http.Client{Timeout: 10 * time.Second},
		metricMap: make(map[string]*prometheus.GaugeVec),
		token:     cfg.Token,
	}
}

func (m *Manager) InitMetrics() {
	for _, req := range m.cfg.Requests {
		for _, metric := range req.Metrics {
			if _, exists := m.metricMap[metric.Name]; !exists {
				gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
					Name: metric.Name,
					Help: metric.Help,
				}, []string{"api_path"})

				prometheus.MustRegister(gauge)
				m.metricMap[metric.Name] = gauge
			}
		}
	}
}

func (m *Manager) Start() {
	log.Printf("Starting Collector. Interval: %s", m.cfg.ScrapeInterval)

	m.scrapeAll()

	ticker := time.NewTicker(m.cfg.GetDuration())
	go func() {
		for range ticker.C {
			m.scrapeAll()
		}
	}()
}

func (m *Manager) scrapeAll() {
	for _, req := range m.cfg.Requests {
		m.fetchAndProcess(req)
	}
}

func (m *Manager) fetchAndProcess(reqCfg config.RequestConfig) {
	url := "https://api.github.com" + reqCfg.ApiPath
	method := reqCfg.Method
	if method == "" {
		method = "GET"
	}

	// Prepare Body (if any)
	var bodyReader io.Reader
	if reqCfg.Body != "" {
		bodyReader = strings.NewReader(reqCfg.Body)
	}

	req, _ := http.NewRequest(method, url, bodyReader)

	if m.token != "" {
		req.Header.Add("Authorization", "Bearer "+m.token)
	}

	// IMPORTANT: GraphQL requires JSON content type
	if method == "POST" {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := m.client.Do(req)
	if err != nil {
		log.Printf("Error fetching %s: %v", url, err)
		return
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	jsonStr := string(body)

	for _, metric := range reqCfg.Metrics {
		val := m.parseValue(jsonStr, metric)
		if gauge, ok := m.metricMap[metric.Name]; ok {
			gauge.WithLabelValues(reqCfg.ApiPath).Set(val)
		}
	}
}

func (m *Manager) parseValue(jsonStr string, metric config.MetricConfig) float64 {
	result := gjson.Get(jsonStr, metric.Path)

	if !result.IsArray() {
		return result.Float()
	}

	var val float64
	results := result.Array()

	switch metric.Aggregate {
	case "count":
		return float64(len(results))
	case "max":
		for _, r := range results {
			if r.Float() > val {
				val = r.Float()
			}
		}
	case "sum": // default
		fallthrough
	default:
		for _, r := range results {
			val += r.Float()
		}
	}
	return val
}
