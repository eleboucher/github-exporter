package cmd

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/eleboucher/github-exporter/internal/collector"
	"github.com/eleboucher/github-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	port       string
	githubUser string
)

var rootCmd = &cobra.Command{
	Use:   "github-exporter",
	Short: "A generic GitHub Prometheus exporter",
	Long:  `Scrapes GitHub API endpoints based on a YAML configuration and exposes them as Prometheus metrics.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cfgFile, githubUser)
		if err != nil {
			log.Fatalf("Error loading config file: %v", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		mgr := collector.NewManager(cfg)
		mgr.InitMetrics()
		mgr.Start(ctx)

		log.Printf("Exporter listening on port %s", port)
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(":"+port, nil); err != nil {
				log.Fatal(err)
			}
		}()
		<-ctx.Done()
		stop()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file path")
	rootCmd.PersistentFlags().StringVar(&githubUser, "github-user", "", "GitHub username")
	rootCmd.PersistentFlags().StringVar(&port, "port", "2112", "port to listen on")
}
