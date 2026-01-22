package cmd

import (
	"log"
	"net/http"

	"github.com/eleboucher/github-exporter/internal/collector"
	"github.com/eleboucher/github-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	port    string
)

var rootCmd = &cobra.Command{
	Use:   "github-exporter",
	Short: "A generic GitHub Prometheus exporter",
	Long:  `Scrapes GitHub API endpoints based on a YAML configuration and exposes them as Prometheus metrics.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			log.Fatalf("Error loading config file: %v", err)
		}

		mgr := collector.NewManager(cfg)
		mgr.InitMetrics()
		mgr.Start()

		log.Printf("Exporter listening on port %s", port)
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file path")
	rootCmd.PersistentFlags().StringVar(&port, "port", "2112", "port to listen on")
}
