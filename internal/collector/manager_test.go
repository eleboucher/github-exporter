package collector

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eleboucher/github-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestParseValue_Float(t *testing.T) {
	m := &Manager{}
	metric := config.MetricConfig{
		Path:      "followers",
		ValueType: config.TypeFloat,
	}

	jsonStr := `{"followers": 42}`
	val := m.parseValue(jsonStr, metric)

	if val != 42.0 {
		t.Errorf("Expected 42.0, got %f", val)
	}
}

func TestParseValue_Date(t *testing.T) {
	m := &Manager{}
	metric := config.MetricConfig{
		Path:      "created_at",
		ValueType: config.TypeDate,
	}

	jsonStr := `{"created_at": "2024-01-15T10:30:00Z"}`
	val := m.parseValue(jsonStr, metric)

	expectedTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	expected := float64(expectedTime.Unix())

	if val != expected {
		t.Errorf("Expected %f, got %f", expected, val)
	}
}

func TestParseValue_AggregateSum(t *testing.T) {
	m := &Manager{}
	metric := config.MetricConfig{
		Path:      "#.stargazers_count",
		Aggregate: config.AggregateSum,
	}

	jsonStr := `[{"stargazers_count": 10}, {"stargazers_count": 20}, {"stargazers_count": 30}]`
	val := m.parseValue(jsonStr, metric)

	if val != 60.0 {
		t.Errorf("Expected 60.0, got %f", val)
	}
}

func TestParseValue_AggregateCount(t *testing.T) {
	m := &Manager{}
	metric := config.MetricConfig{
		Path:      "#.stargazers_count",
		Aggregate: config.AggregateCount,
	}

	jsonStr := `[{"stargazers_count": 10}, {"stargazers_count": 20}, {"stargazers_count": 30}]`
	val := m.parseValue(jsonStr, metric)

	if val != 3.0 {
		t.Errorf("Expected 3.0, got %f", val)
	}
}

func TestParseValue_AggregateMax(t *testing.T) {
	m := &Manager{}
	metric := config.MetricConfig{
		Path:      "#.stargazers_count",
		Aggregate: config.AggregateMax,
	}

	jsonStr := `[{"stargazers_count": 10}, {"stargazers_count": 30}, {"stargazers_count": 20}]`
	val := m.parseValue(jsonStr, metric)

	if val != 30.0 {
		t.Errorf("Expected 30.0, got %f", val)
	}
}

func TestParseValue_InvalidDate(t *testing.T) {
	m := &Manager{}
	metric := config.MetricConfig{
		Path:      "created_at",
		ValueType: config.TypeDate,
	}

	jsonStr := `{"created_at": "invalid-date"}`
	val := m.parseValue(jsonStr, metric)

	if val != 0 {
		t.Errorf("Expected 0 for invalid date, got %f", val)
	}
}

func TestDescribe(t *testing.T) {
	cfg := &config.Config{
		GithubAPIURL: "https://api.github.com",
		Requests: []config.RequestConfig{
			{
				ApiPath: "/users/test",
				Metrics: []config.MetricConfig{
					{
						Name: "github_followers",
						Path: "followers",
						Help: "Total followers",
					},
					{
						Name: "github_repos",
						Path: "public_repos",
						Help: "Public repositories",
					},
				},
			},
		},
	}

	m := NewManager(cfg)
	ch := make(chan *prometheus.Desc, 10)
	go func() {
		m.Describe(ch)
		close(ch)
	}()

	count := 0
	for range ch {
		count++
	}

	if count != 2 {
		t.Errorf("Expected 2 descriptors, got %d", count)
	}
}

func TestCollect_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify cache-busting headers
		if r.Header.Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
			t.Error("Missing Cache-Control header")
		}
		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Error("Missing X-GitHub-Api-Version header")
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("Missing or incorrect Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, `{"followers": 100, "public_repos": 50}`); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		GithubAPIURL: server.URL,
		Token:        "test-token",
		Requests: []config.RequestConfig{
			{
				ApiPath: "/users/test",
				Metrics: []config.MetricConfig{
					{
						Name: "github_followers",
						Path: "followers",
						Help: "Total followers",
					},
				},
			},
		},
	}

	m := NewManager(cfg)
	ch := make(chan prometheus.Metric, 10)
	go func() {
		m.Collect(ch)
		close(ch)
	}()

	metricCount := 0
	for metric := range ch {
		metricCount++
		// Verify the metric value
		var metricDTO dto.Metric
		if err := metric.Write(&metricDTO); err != nil {
			t.Errorf("Failed to write metric: %v", err)
		}
		if metricDTO.GetGauge().GetValue() != 100.0 {
			t.Errorf("Expected metric value 100.0, got %f", metricDTO.GetGauge().GetValue())
		}
	}

	if metricCount != 1 {
		t.Errorf("Expected 1 metric, got %d", metricCount)
	}
}

func TestCollect_WithLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, `[
			{"type": "PushEvent", "repo": {"name": "user/repo1"}, "created_at": "2024-01-15T10:30:00Z"},
			{"type": "IssueEvent", "repo": {"name": "user/repo2"}, "created_at": "2024-01-14T10:30:00Z"}
		]`); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		GithubAPIURL: server.URL,
		Requests: []config.RequestConfig{
			{
				ApiPath: "/users/test/events",
				Metrics: []config.MetricConfig{
					{
						Name:      "github_last_push_info",
						Path:      `#(type=="PushEvent").created_at`,
						Help:      "Last push event",
						ValueType: config.TypeDate,
						Labels: map[string]string{
							"repo": `#(type=="PushEvent").repo.name`,
							"type": `#(type=="PushEvent").type`,
						},
					},
				},
			},
		},
	}

	m := NewManager(cfg)
	ch := make(chan prometheus.Metric, 10)
	go func() {
		m.Collect(ch)
		close(ch)
	}()

	for metric := range ch {
		var metricDTO dto.Metric
		if err := metric.Write(&metricDTO); err != nil {
			t.Errorf("Failed to write metric: %v", err)
		}

		// Check labels
		labels := make(map[string]string)
		for _, label := range metricDTO.GetLabel() {
			labels[label.GetName()] = label.GetValue()
		}

		if labels["repo"] != "user/repo1" {
			t.Errorf("Expected repo label 'user/repo1', got '%s'", labels["repo"])
		}
		if labels["type"] != "PushEvent" {
			t.Errorf("Expected type label 'PushEvent', got '%s'", labels["type"])
		}
	}
}

func TestCollect_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		GithubAPIURL: server.URL,
		Requests: []config.RequestConfig{
			{
				ApiPath: "/users/test",
				Metrics: []config.MetricConfig{
					{
						Name: "github_followers",
						Path: "followers",
						Help: "Total followers",
					},
				},
			},
		},
	}

	m := NewManager(cfg)
	ch := make(chan prometheus.Metric, 10)
	go func() {
		m.Collect(ch)
		close(ch)
	}()

	// Should not send any metrics on error
	metricCount := 0
	for range ch {
		metricCount++
	}

	if metricCount != 0 {
		t.Errorf("Expected 0 metrics on error, got %d", metricCount)
	}
}

func TestCollect_POSTRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Missing Content-Type header for POST")
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "query") {
			t.Error("POST body doesn't contain expected query")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, `{"data": {"user": {"contributionsCollection": {"contributionCalendar": {"totalContributions": 365}}}}}`); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		GithubAPIURL: server.URL,
		Requests: []config.RequestConfig{
			{
				ApiPath: "/graphql",
				Method:  "POST",
				Body:    `{"query": "{ user { contributionsCollection { contributionCalendar { totalContributions } } } }"}`,
				Metrics: []config.MetricConfig{
					{
						Name: "github_contributions",
						Path: "data.user.contributionsCollection.contributionCalendar.totalContributions",
						Help: "Total contributions",
					},
				},
			},
		},
	}

	m := NewManager(cfg)
	ch := make(chan prometheus.Metric, 10)
	go func() {
		m.Collect(ch)
		close(ch)
	}()

	metricCount := 0
	for metric := range ch {
		metricCount++
		var metricDTO dto.Metric
		if err := metric.Write(&metricDTO); err != nil {
			t.Errorf("Failed to write metric: %v", err)
		}
		if metricDTO.GetGauge().GetValue() != 365.0 {
			t.Errorf("Expected metric value 365.0, got %f", metricDTO.GetGauge().GetValue())
		}
	}

	if metricCount != 1 {
		t.Errorf("Expected 1 metric, got %d", metricCount)
	}
}

func TestHTTPTransport_DisableKeepAlives(t *testing.T) {
	cfg := &config.Config{
		GithubAPIURL: "https://api.github.com",
	}

	m := NewManager(cfg)

	transport, ok := m.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	if !transport.DisableKeepAlives {
		t.Error("Expected DisableKeepAlives to be true")
	}
}
