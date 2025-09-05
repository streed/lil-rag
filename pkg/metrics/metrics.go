package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Success labels for metrics
const (
	SuccessTrue  = "true"
	SuccessFalse = "false"
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

	// LLM token usage metrics - comprehensive tracking
	LLMTokensUsed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_llm_tokens_used_total",
		Help: "Estimated total LLM tokens used (input + output)",
	}, []string{"operation", "model", "direction"}) // direction: input/output

	// Token usage by operation type
	EmbeddingTokensUsed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_embedding_tokens_total",
		Help: "Total tokens sent for embedding generation",
	}, []string{"model", "cache_hit"})

	ChatTokensUsed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_chat_tokens_total",
		Help: "Total tokens used in chat operations",
	}, []string{"model", "direction"}) // direction: input/output

	QueryOptimizationTokensUsed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_query_optimization_tokens_total",
		Help: "Total tokens used in query optimization",
	}, []string{"model", "direction"}) // direction: input/output

	ImageOCRTokensUsed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_image_ocr_tokens_total",
		Help: "Total tokens used in image OCR operations",
	}, []string{"model", "direction"}) // direction: input/output

	DocumentTokensProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lilrag_document_tokens_processed_total",
		Help: "Total tokens processed during document indexing",
	}, []string{"document_type"})

	// Token efficiency metrics
	TokenCacheHitRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lilrag_token_cache_hit_rate",
		Help: "Cache hit rate for token operations (0-1)",
	}, []string{"operation_type"})

	// Current token usage per session/model
	ActiveTokenUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lilrag_active_token_usage",
		Help: "Current estimated token usage per model",
	}, []string{"model", "operation_type"})

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
	successLabel := SuccessFalse
	if success {
		successLabel = SuccessTrue
	}

	SearchRequestDuration.WithLabelValues(successLabel).Observe(duration.Seconds())
	SearchRequestsTotal.WithLabelValues(successLabel).Inc()

	if success {
		SearchResultsFound.WithLabelValues().Observe(float64(resultsFound))
	}
}

func RecordIndexingRequest(duration time.Duration, success bool, characterCount int) {
	successLabel := SuccessFalse
	if success {
		successLabel = SuccessTrue
	}

	IndexingRequestDuration.WithLabelValues(successLabel).Observe(duration.Seconds())
	IndexingRequestsTotal.WithLabelValues(successLabel).Inc()

	if success {
		DocumentCharactersIndexed.WithLabelValues().Observe(float64(characterCount))
	}
}

func RecordChatRequest(duration time.Duration, success bool, sourcesCount, responseLength int) {
	successLabel := SuccessFalse
	if success {
		successLabel = SuccessTrue
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
	LLMTokensUsed.WithLabelValues(operation, model, "unknown").Add(float64(estimatedTokens))
}

// Enhanced token tracking functions
func RecordEmbeddingTokens(model, text string, cacheHit bool) int {
	estimatedTokens := estimateTokens(text)
	cacheHitStr := "false"
	if cacheHit {
		cacheHitStr = "true"
	}
	EmbeddingTokensUsed.WithLabelValues(model, cacheHitStr).Add(float64(estimatedTokens))
	LLMTokensUsed.WithLabelValues("embedding", model, "input").Add(float64(estimatedTokens))
	return estimatedTokens
}

func RecordChatInputTokens(model, text string) int {
	estimatedTokens := estimateTokens(text)
	ChatTokensUsed.WithLabelValues(model, "input").Add(float64(estimatedTokens))
	LLMTokensUsed.WithLabelValues("chat", model, "input").Add(float64(estimatedTokens))
	return estimatedTokens
}

func RecordChatOutputTokens(model, text string) int {
	estimatedTokens := estimateTokens(text)
	ChatTokensUsed.WithLabelValues(model, "output").Add(float64(estimatedTokens))
	LLMTokensUsed.WithLabelValues("chat", model, "output").Add(float64(estimatedTokens))
	return estimatedTokens
}

func RecordQueryOptimizationTokens(model, inputQuery, outputQuery string) (inputTokens, outputTokens int) {
	inputTokens = estimateTokens(inputQuery)
	outputTokens = estimateTokens(outputQuery)
	QueryOptimizationTokensUsed.WithLabelValues(model, "input").Add(float64(inputTokens))
	QueryOptimizationTokensUsed.WithLabelValues(model, "output").Add(float64(outputTokens))
	LLMTokensUsed.WithLabelValues("query_optimization", model, "input").Add(float64(inputTokens))
	LLMTokensUsed.WithLabelValues("query_optimization", model, "output").Add(float64(outputTokens))
	return inputTokens, outputTokens
}

func RecordImageOCRTokens(model, prompt, extractedText string) (inputTokens, outputTokens int) {
	inputTokens = estimateTokens(prompt)
	outputTokens = estimateTokens(extractedText)
	ImageOCRTokensUsed.WithLabelValues(model, "input").Add(float64(inputTokens))
	ImageOCRTokensUsed.WithLabelValues(model, "output").Add(float64(outputTokens))
	LLMTokensUsed.WithLabelValues("image_ocr", model, "input").Add(float64(inputTokens))
	LLMTokensUsed.WithLabelValues("image_ocr", model, "output").Add(float64(outputTokens))
	return inputTokens, outputTokens
}

func RecordDocumentTokens(documentType string, totalTokens int) {
	DocumentTokensProcessed.WithLabelValues(documentType).Add(float64(totalTokens))
}

func UpdateTokenCacheHitRate(operationType string, hitRate float64) {
	TokenCacheHitRate.WithLabelValues(operationType).Set(hitRate)
}

func UpdateActiveTokenUsage(model, operationType string, tokenCount int) {
	ActiveTokenUsage.WithLabelValues(model, operationType).Set(float64(tokenCount))
}

// Helper function for consistent token estimation
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// More sophisticated estimation:
	// - ~4 characters per token for English
	// - Account for punctuation and special characters
	// - Add a small buffer for model-specific variations
	baseTokens := len(text) / 4
	// Add 10% buffer for tokenization variations
	return int(float64(baseTokens) * 1.1)
}

func RecordQueryOptimization(duration time.Duration, success bool) {
	successLabel := SuccessFalse
	if success {
		successLabel = SuccessTrue
	}

	QueryOptimizationDuration.WithLabelValues(successLabel).Observe(duration.Seconds())
	QueryOptimizationRequestsTotal.WithLabelValues(successLabel).Inc()
}

func UpdateSystemMetrics(dbSizeBytes, vectorIndexEntries int64) {
	DatabaseSize.WithLabelValues().Set(float64(dbSizeBytes))
	VectorIndexSize.WithLabelValues().Set(float64(vectorIndexEntries))
}
