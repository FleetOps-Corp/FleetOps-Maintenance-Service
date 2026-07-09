package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fleetops/maintenance/internal/platform/metrics"
)

// PrometheusMetrics records business and HTTP metrics for each request.
func PrometheusMetrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/metrics" || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(recorder, r)

			path := strings.TrimSpace(r.URL.Path)
			if path == "" {
				path = "/"
			}

			met := metrics.Default()
			met.HTTPRequestsTotal.WithLabelValues(strings.ToUpper(r.Method), path, strconv.Itoa(recorder.statusCode)).Inc()
			met.HTTPRequestDurationSeconds.WithLabelValues(strings.ToUpper(r.Method), path).Observe(time.Since(start).Seconds())
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
