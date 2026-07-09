package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds business and HTTP metrics exposed to Prometheus.
type Metrics struct {
	HTTPRequestsTotal          *prometheus.CounterVec
	HTTPRequestDurationSeconds *prometheus.HistogramVec
	MaintenanceCreatedTotal    *prometheus.CounterVec
	MaintenanceErrorsTotal     *prometheus.CounterVec
	QueueProcessedTotal        prometheus.Counter
	QueueErrorsTotal           prometheus.Counter
	VehicleClientRequests      *prometheus.CounterVec
	VehicleClientDuration      *prometheus.HistogramVec
}

var defaultMetrics = New()

// Default returns the singleton metrics registry used by the service.
func Default() *Metrics {
	return defaultMetrics
}

// New creates and registers Prometheus collectors.
func New() *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled by the service",
		}, []string{"method", "path", "status_code"}),
		HTTPRequestDurationSeconds: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		MaintenanceCreatedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "maintenance_created_total",
			Help: "Total number of maintenance records created",
		}, []string{"type"}),
		MaintenanceErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "maintenance_errors_total",
			Help: "Total number of maintenance operation errors",
		}, []string{"operation"}),
		QueueProcessedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "maintenance_queue_processed_total",
			Help: "Total number of queued maintenance items processed",
		}),
		QueueErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "maintenance_queue_errors_total",
			Help: "Total number of queue processing errors",
		}),
		VehicleClientRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "vehicle_client_requests_total",
			Help: "Total number of vehicle service requests",
		}, []string{"operation"}),
		VehicleClientDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vehicle_client_duration_seconds",
			Help:    "Vehicle service request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation"}),
	}
}
