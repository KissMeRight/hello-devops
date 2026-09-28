package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	version   = "0.1.0"
	startTime = time.Now()

	ready       atomic.Bool  // สถานะที่ readinessProbe จะเช็ค
	reqTotal    atomic.Int64 // ตัวนับสำหรับ /metrics
	reqErrTotal atomic.Int64
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// middleware: log ทุก request + นับ metric
func withLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqTotal.Add(1)
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next(rec, r)
		if rec.status >= 500 {
			reqErrTotal.Add(1)
		}
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	}
}

func main() {
	// config มาจาก environment (ConfigMap / Secret)
	port := env("PORT", "8080")
	greeting := env("GREETING", "Hello")
	appEnv := env("APP_ENV", "local")
	logLevel := env("LOG_LEVEL", "info")
	apiKey := env("API_KEY", "")
	podName := env("POD_NAME", "unknown")
	nodeName := env("NODE_NAME", "unknown")

	// structured logging แบบ JSON
	lvl := slog.LevelInfo
	switch strings.ToLower(logLevel) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))

	mux := http.NewServeMux()

	mux.HandleFunc("/", withLogging(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		host, _ := os.Hostname()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message":            fmt.Sprintf("%s from Kubernetes!", greeting),
			"version":            version,
			"env":                appEnv,
			"hostname":           host,
			"pod_name":           podName,
			"node_name":          nodeName,
			"uptime_sec":         int(time.Since(startTime).Seconds()),
			"api_key_configured": apiKey != "", // ห้ามแสดง secret จริงเด็ดขาด
			"time":               time.Now().Format(time.RFC3339),
		})
	}))

	// liveness: "ยังมีชีวิตไหม" ต้องเบา ห้ามเช็ค dependency ภายนอก
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	// readiness: "พร้อมรับ traffic ไหม"
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "not ready")
			return
		}
		fmt.Fprintln(w, "ready")
	})

	// metrics แบบ Prometheus
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# TYPE app_requests_total counter\n")
		fmt.Fprintf(w, "app_requests_total{env=%q} %d\n", appEnv, reqTotal.Load())
		fmt.Fprintf(w, "# TYPE app_request_errors_total counter\n")
		fmt.Fprintf(w, "app_request_errors_total{env=%q} %d\n", appEnv, reqErrTotal.Load())
		fmt.Fprintf(w, "# TYPE app_uptime_seconds gauge\n")
		fmt.Fprintf(w, "app_uptime_seconds %.0f\n", time.Since(startTime).Seconds())
	})

	// จำลอง error ไว้ทดสอบ alert
	mux.HandleFunc("/boom", withLogging(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "intentional error", http.StatusInternalServerError)
	}))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // กัน Slowloris
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// จำลอง warm-up 3 วินาที
	go func() {
		time.Sleep(3 * time.Second)
		ready.Store(true)
		slog.Info("application is ready")
	}()

	go func() {
		slog.Info("server starting", "port", port, "version", version, "env", appEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("SIGTERM received, draining...")

	ready.Store(false)          // 1) ประกาศว่าไม่พร้อม -> ถูกถอดจาก endpoint
	time.Sleep(3 * time.Second) // 2) รอให้ระบบ network อัปเดต

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil { // 3) รอ request ที่ค้างอยู่ให้จบ
		slog.Error("forced shutdown", "err", err)
	}
	slog.Info("server stopped cleanly")
}
