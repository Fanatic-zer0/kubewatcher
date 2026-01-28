package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"k8watch/internal/api"
	"k8watch/internal/storage"
	"k8watch/internal/watcher"
)

func main() {
	// Parse flags
	kubeconfig := flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "Path to kubeconfig file")
	dbPath := flag.String("db", "./events.db", "Path to SQLite database file")
	addr := flag.String("addr", ":8080", "HTTP server address")
	retentionDays := flag.Int("retention", 60, "Event retention period in days")
	slackWebhook := flag.String("slack-webhook", os.Getenv("SLACK_WEBHOOK_URL"), "Slack webhook URL for notifications")
	flag.Parse()

	log.Println("Starting K8Watch - Kubernetes Change Tracker")
	log.Printf("Kubeconfig: %s", *kubeconfig)
	log.Printf("Database: %s", *dbPath)
	log.Printf("Server: %s", *addr)
	log.Printf("Retention: %d days", *retentionDays)

	// Initialize storage
	store, err := storage.NewStorage(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initial cleanup of old events
	if deleted, err := store.CleanupOldEvents(*retentionDays); err != nil {
		log.Printf("Warning: Failed to cleanup old events: %v", err)
	} else if deleted > 0 {
		log.Printf("Cleaned up %d events older than %d days", deleted, *retentionDays)
	}

	// Start periodic cleanup (daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if deleted, err := store.CleanupOldEvents(*retentionDays); err != nil {
				log.Printf("Warning: Periodic cleanup failed: %v", err)
			} else if deleted > 0 {
				log.Printf("Periodic cleanup: removed %d old events", deleted)
			}
		}
	}()

	// Initialize watcher
	w, err := watcher.NewWatcher(*kubeconfig, store, *slackWebhook)
	if err != nil {
		log.Fatalf("Failed to initialize watcher: %v", err)
	}

	// Start watching
	if err := w.Start(); err != nil {
		log.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Start API server
	server := api.NewServer(store)
	go func() {
		if err := server.Start(*addr); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	log.Printf("K8Watch is running! Access the UI at http://localhost%s", *addr)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down gracefully...")
}
