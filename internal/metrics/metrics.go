package metrics

import (
    "net/http"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    // Query metrics
    QueriesTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "chronosdb_queries_total",
            Help: "Total number of queries executed",
        },
        []string{"type", "status"},
    )

    QueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "chronosdb_query_duration_seconds",
            Help:    "Duration of query execution",
            Buckets: prometheus.DefBuckets,
        },
        []string{"type"},
    )

    // Storage metrics
    NodesTotal = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "chronosdb_nodes_total",
            Help: "Total number of nodes in database",
        },
    )

    EdgesTotal = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "chronosdb_edges_total",
            Help: "Total number of edges in database",
        },
    )

    // Request metrics
    HTTPRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "chronosdb_http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )

    HTTPRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "chronosdb_http_request_duration_seconds",
            Help:    "Duration of HTTP requests",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )

    // Kafka metrics
    KafkaMessagesTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "chronosdb_kafka_messages_total",
            Help: "Total Kafka messages processed",
        },
    )

    KafkaErrorsTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "chronosdb_kafka_errors_total",
            Help: "Total Kafka processing errors",
        },
    )

    // Cache metrics
    CacheHits = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "chronosdb_cache_hits_total",
            Help: "Total cache hits",
        },
    )

    CacheMisses = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "chronosdb_cache_misses_total",
            Help: "Total cache misses",
        },
    )
)

// RecordQuery records metrics for a query execution
func RecordQuery(queryType string, duration time.Duration, success bool) {
    status := "success"
    if !success {
        status = "error"
    }
    QueriesTotal.WithLabelValues(queryType, status).Inc()
    QueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// RecordHTTPRequest records metrics for HTTP requests
func RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
    status := http.StatusText(statusCode)
    HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
    HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordKafkaMessage records Kafka processing
func RecordKafkaMessage(success bool) {
    KafkaMessagesTotal.Inc()
    if !success {
        KafkaErrorsTotal.Inc()
    }
}

// RecordCache records cache hit/miss
func RecordCache(hit bool) {
    if hit {
        CacheHits.Inc()
    } else {
        CacheMisses.Inc()
    }
}

// UpdateStorageMetrics updates node/edge counts
func UpdateStorageMetrics(nodes, edges int) {
    NodesTotal.Set(float64(nodes))
    EdgesTotal.Set(float64(edges))
}

// MetricsHandler returns HTTP handler for Prometheus metrics
func MetricsHandler() http.Handler {
    return promhttp.Handler()
}
