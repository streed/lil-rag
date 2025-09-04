package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint", "status_code"})

	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "endpoint", "status_code"})

	// Search and indexing metrics
	SearchRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_search_duration_seconds", 
		Help:    "Duration of search operations in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"success"})

	SearchRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_search_requests_total",
		Help: "Total number of search requests",
	}, []string{"success"})

	SearchResultsFound = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_search_results_found",
		Help:    "Number of search results returned",
		Buckets: []float64{0, 1, 2, 5, 10, 20, 50, 100},
	}, []string{})

	IndexingRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_indexing_duration_seconds",
		Help:    "Duration of document indexing operations in seconds",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
	}, []string{"success"})

	IndexingRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_indexing_requests_total",
		Help: "Total number of indexing requests",
	}, []string{"success"})

	DocumentsIndexed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lilrag_documents_total",
		Help: "Total number of documents stored in the system",
	}, []string{})

	DocumentCharactersIndexed = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_document_characters_indexed",
		Help:    "Number of characters in indexed documents",
		Buckets: []float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000},
	}, []string{})

	// Chat/LLM metrics
	ChatRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_chat_duration_seconds",
		Help:    "Duration of chat/LLM operations in seconds", 
		Buckets: []float64{0.5, 1, 2.5, 5, 10, 15, 30, 60, 120},
	}, []string{"success"})

	ChatRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_chat_requests_total",
		Help: "Total number of chat requests",
	}, []string{"success"})

	ChatSourcesRetrieved = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_chat_sources_retrieved",
		Help:    "Number of sources retrieved for chat context",
		Buckets: []float64{0, 1, 2, 5, 10, 15, 20},
	}, []string{})

	ChatResponseLength = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_chat_response_length_characters",
		Help:    "Length of chat responses in characters",
		Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 20000},
	}, []string{})

	// LLM token usage (estimated - would need Ollama integration for actual tokens)
	LLMTokensUsed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_llm_tokens_used_total",
		Help: "Estimated total LLM tokens used (input + output)",
	}, []string{"operation", "model"})

	// Query optimization metrics
	QueryOptimizationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lilrag_query_optimization_duration_seconds",
		Help:    "Duration of query optimization operations in seconds",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"success"})

	QueryOptimizationRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_query_optimization_requests_total",
		Help: "Total number of query optimization requests",
	}, []string{"success"})

	// System metrics
	DatabaseSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lilrag_database_size_bytes",
		Help: "Size of the database file in bytes",
	}, []string{})

	VectorIndexSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lilrag_vector_index_entries",
		Help: "Number of entries in the vector index",
	}, []string{})
)

// Helper functions for recording metrics with timing
func RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	status := prometheus.Labels{
		"method":      method,
		"endpoint":    endpoint, 
		"status_code": strconv.Itoa(statusCode),
	}
	HTTPRequestDuration.With(status).Observe(duration.Seconds())
	HTTPRequestsTotal.With(status).Inc()
}

func RecordSearchRequest(duration time.Duration, success bool, resultsFound int) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}
	
	SearchRequestDuration.WithLabelValues(successLabel).Observe(duration.Seconds())
	SearchRequestsTotal.WithLabelValues(successLabel).Inc()
	
	if success {
		SearchResultsFound.WithLabelValues().Observe(float64(resultsFound))
	}
}

func RecordIndexingRequest(duration time.Duration, success bool, characterCount int) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}
	
	IndexingRequestDuration.WithLabelValues(successLabel).Observe(duration.Seconds())
	IndexingRequestsTotal.WithLabelValues(successLabel).Inc()
	
	if success {
		DocumentCharactersIndexed.WithLabelValues().Observe(float64(characterCount))
	}
}

func RecordChatRequest(duration time.Duration, success bool, sourcesCount int, responseLength int) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}
	
	ChatRequestDuration.WithLabelValues(successLabel).Observe(duration.Seconds())
	ChatRequestsTotal.WithLabelValues(successLabel).Inc()
	
	if success {
		ChatSourcesRetrieved.WithLabelValues().Observe(float64(sourcesCount))
		ChatResponseLength.WithLabelValues().Observe(float64(responseLength))
	}
}

func UpdateDocumentCount(count int) {
	DocumentsIndexed.WithLabelValues().Set(float64(count))
}

func EstimateAndRecordTokens(operation, model, text string) {
	// Rough estimation: ~4 characters per token for English text
	estimatedTokens := len(text) / 4
	LLMTokensUsed.WithLabelValues(operation, model).Add(float64(estimatedTokens))
}

func RecordQueryOptimization(duration time.Duration, success bool) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}
	
	QueryOptimizationDuration.WithLabelValues(successLabel).Observe(duration.Seconds())
	QueryOptimizationRequestsTotal.WithLabelValues(successLabel).Inc()
}

func UpdateSystemMetrics(dbSizeBytes, vectorIndexEntries int64) {
	DatabaseSize.WithLabelValues().Set(float64(dbSizeBytes))
	VectorIndexSize.WithLabelValues().Set(float64(vectorIndexEntries))
}