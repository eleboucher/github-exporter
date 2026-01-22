package collector

import (
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eleboucher/github-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

type Manager struct {
	cfg          *config.Config
	client       *http.Client
	metricMap    map[string]*prometheus.GaugeVec
	token        string
	scrapeErrors prometheus.Counter
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		client:    &http.Client{Timeout: 10 * time.Second},
		metricMap: make(map[string]*prometheus.GaugeVec),
		token:     cfg.Token,
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "github_exporter_scrape_errors_total",
			Help: "Total number of scrape errors",
		}),
	}
}

func (m *Manager) InitMetrics() {
	for _, req := range m.cfg.Requests {
		for _, metric := range req.Metrics {
			gauge := prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: metric.Name,
					Help: metric.Help,
				},
				[]string{"api_path"},
			)

			prometheus.MustRegister(gauge)
			m.metricMap[metric.Name] = gauge
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
	var wg sync.WaitGroup
	for _, req := range m.cfg.Requests {
		wg.Add(1)
		go func(r config.RequestConfig) {
			defer wg.Done()
			m.fetchAndProcess(r)
		}(req)
	}
	wg.Wait()
}

func (m *Manager) fetchAndProcess(reqCfg config.RequestConfig) {
	path := strings.TrimLeft(reqCfg.ApiPath, "/")
	url := m.cfg.GithubAPIURL + "/" + path
	method := reqCfg.Method
	if method == "" {
		method = "GET"
	}

	// Prepare Body (if any)
	var bodyReader io.Reader
	if reqCfg.Body != "" {
		bodyReader = strings.NewReader(reqCfg.Body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		log.Printf("Error creating request for %s: %v", url, err)
		m.scrapeErrors.Inc()
		return
	}

	req.Header.Set("User-Agent", "eleboucher-github-exporter/1.0")

	if m.token != "" {
		req.Header.Add("Authorization", "Bearer "+m.token)
	}

	if method == "POST" {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := m.client.Do(req)
	if err != nil {
		log.Printf("Error fetching %s: %v", url, err)
		m.scrapeErrors.Inc()
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Non-200 status code from %s: %d", url, resp.StatusCode)
		m.scrapeErrors.Inc()
		return
	}

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
		if len(results) > 0 {
			val = results[0].Float()
			for _, r := range results[1:] {
				if r.Float() > val {
					val = r.Float()
				}
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
