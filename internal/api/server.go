package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"k8watch/internal/storage"

	"github.com/gorilla/mux"
)

type Server struct {
	storage    *storage.Storage
	router     *mux.Router
	statsCache *cacheEntry
	cacheMutex sync.RWMutex
}

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

const cacheTTL = 10 * time.Second

// NewServer creates a new API server
func NewServer(storage *storage.Storage) *Server {
	s := &Server{
		storage: storage,
		router:  mux.NewRouter(),
	}
	s.setupRoutes()
	return s
}

// setupRoutes configures API routes
func (s *Server) setupRoutes() {
	// API routes (must come before static files)
	api := s.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/events", s.getEvents).Methods("GET")
	api.HandleFunc("/timeline/{namespace}/{kind}/{name}", s.getTimeline).Methods("GET")
	api.HandleFunc("/stats", s.getStats).Methods("GET")
	api.HandleFunc("/cleanup", s.cleanupOldEvents).Methods("POST")

	// Static files (catch-all, must be last)
	s.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web")))
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	log.Printf("Starting API server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// getEvents returns filtered events
func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query()
	filter := storage.Filter{
		Namespace: query.Get("namespace"),
		Kind:      query.Get("kind"),
		Name:      query.Get("name"),
		Action:    query.Get("action"),
		Limit:     50, // default page size
	}

	// Parse time filters
	if startTime := query.Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = t
		}
	}
	if endTime := query.Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = t
		}
	}

	// Parse limit and offset (pagination)
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = l
		}
	}
	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	events, err := s.storage.GetEvents(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	totalCount, err := s.storage.GetTotalCount(filter)
	if err != nil {
		log.Printf("Warning: failed to get total count: %v", err)
		totalCount = int64(len(events))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"events":      events,
		"count":       len(events),
		"total_count": totalCount,
		"offset":      filter.Offset,
		"limit":       filter.Limit,
	})
}

// getTimeline returns timeline for a specific resource
func (s *Server) getTimeline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	namespace := vars["namespace"]
	kind := vars["kind"]
	name := vars["name"]

	timeline, err := s.storage.GetTimeline(namespace, kind, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"timeline": timeline,
		"count":    len(timeline),
	})
}

// getStats returns dashboard statistics
func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check cache
	s.cacheMutex.RLock()
	if s.statsCache != nil && time.Since(s.statsCache.timestamp) < cacheTTL {
		json.NewEncoder(w).Encode(s.statsCache.data)
		s.cacheMutex.RUnlock()
		return
	}
	s.cacheMutex.RUnlock()

	// Fetch fresh data
	stats, err := s.storage.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update cache
	s.cacheMutex.Lock()
	s.statsCache = &cacheEntry{
		data:      stats,
		timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	json.NewEncoder(w).Encode(stats)
}

// cleanupOldEvents manually triggers cleanup of old events
func (s *Server) cleanupOldEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	retentionDays := 60 // default
	if days := r.URL.Query().Get("days"); days != "" {
		if d, err := strconv.Atoi(days); err == nil && d > 0 {
			retentionDays = d
		}
	}

	deleted, err := s.storage.CleanupOldEvents(retentionDays)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted":        deleted,
		"retention_days": retentionDays,
		"message":        "Cleanup completed successfully",
	})
}
