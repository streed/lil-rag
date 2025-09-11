package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRecordIndexingRequest(t *testing.T) {
	tests := []struct {
		name        string
		duration    time.Duration
		success     bool
		textLength  int
		expectError bool
	}{
		{
			name:        "successful indexing",
			duration:    100 * time.Millisecond,
			success:     true,
			textLength:  1000,
			expectError: false,
		},
		{
			name:        "failed indexing",
			duration:    50 * time.Millisecond,
			success:     false,
			textLength:  500,
			expectError: false,
		},
		{
			name:        "zero duration",
			duration:    0,
			success:     true,
			textLength:  100,
			expectError: false,
		},
		{
			name:        "large text",
			duration:    200 * time.Millisecond,
			success:     true,
			textLength:  100000,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric
			RecordIndexingRequest(tt.duration, tt.success, tt.textLength)

			// Verify the metrics were recorded
			// Note: We can't easily verify the exact values without complex prometheus testing,
			// but we can at least verify the function executes without panicking
			if tt.expectError {
				t.Error("Expected error but function completed successfully")
			}
		})
	}
}

func TestRecordSearchRequest(t *testing.T) {
	tests := []struct {
		name        string
		duration    time.Duration
		success     bool
		resultCount int
	}{
		{
			name:        "successful search with results",
			duration:    50 * time.Millisecond,
			success:     true,
			resultCount: 5,
		},
		{
			name:        "successful search with no results",
			duration:    25 * time.Millisecond,
			success:     true,
			resultCount: 0,
		},
		{
			name:        "failed search",
			duration:    10 * time.Millisecond,
			success:     false,
			resultCount: 0,
		},
		{
			name:        "search with many results",
			duration:    150 * time.Millisecond,
			success:     true,
			resultCount: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordSearchRequest(tt.duration, tt.success, tt.resultCount)
			// Verify function executes without panicking
		})
	}
}

func TestRecordChatRequest(t *testing.T) {
	tests := []struct {
		name           string
		duration       time.Duration
		success        bool
		sourceCount    int
		responseLength int
	}{
		{
			name:           "successful chat with sources",
			duration:       500 * time.Millisecond,
			success:        true,
			sourceCount:    3,
			responseLength: 250,
		},
		{
			name:           "successful chat without sources",
			duration:       200 * time.Millisecond,
			success:        true,
			sourceCount:    0,
			responseLength: 100,
		},
		{
			name:           "failed chat",
			duration:       100 * time.Millisecond,
			success:        false,
			sourceCount:    0,
			responseLength: 0,
		},
		{
			name:           "chat with long response",
			duration:       1 * time.Second,
			success:        true,
			sourceCount:    10,
			responseLength: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordChatRequest(tt.duration, tt.success, tt.sourceCount, tt.responseLength)
			// Verify function executes without panicking
		})
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		endpoint   string
		statusCode int
		duration   time.Duration
	}{
		{
			name:       "GET health endpoint",
			method:     "GET",
			endpoint:   "/api/health",
			statusCode: 200,
			duration:   10 * time.Millisecond,
		},
		{
			name:       "POST index endpoint",
			method:     "POST",
			endpoint:   "/api/index",
			statusCode: 201,
			duration:   100 * time.Millisecond,
		},
		{
			name:       "GET search endpoint",
			method:     "GET",
			endpoint:   "/api/search",
			statusCode: 200,
			duration:   50 * time.Millisecond,
		},
		{
			name:       "error response",
			method:     "POST",
			endpoint:   "/api/index",
			statusCode: 500,
			duration:   25 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordHTTPRequest(tt.method, tt.endpoint, tt.statusCode, tt.duration)
			// Verify function executes without panicking
		})
	}
}

func TestRecordEmbeddingTokens(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		text     string
		cacheHit bool
	}{
		{
			name:     "document embedding",
			model:    "nomic-embed-text",
			text:     "This is a test document for embedding",
			cacheHit: false,
		},
		{
			name:     "query embedding with cache hit",
			model:    "nomic-embed-text",
			text:     "test query",
			cacheHit: true,
		},
		{
			name:     "empty text",
			model:    "nomic-embed-text",
			text:     "",
			cacheHit: false,
		},
		{
			name:     "large text",
			model:    "nomic-embed-text",
			text:     "This is a very long document that contains a lot of text and should result in a higher token count for testing purposes. " + string(make([]byte, 1000)),
			cacheHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := RecordEmbeddingTokens(tt.model, tt.text, tt.cacheHit)
			if tokens < 0 {
				t.Errorf("Expected non-negative token count, got %d", tokens)
			}
		})
	}
}

func TestUpdateDocumentCount(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{
			name:  "zero documents",
			count: 0,
		},
		{
			name:  "single document",
			count: 1,
		},
		{
			name:  "many documents",
			count: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateDocumentCount(tt.count)
			// Verify function executes without panicking
		})
	}
}

func TestMetricsConstants(t *testing.T) {
	// Test that our success constants are correctly defined
	if SuccessTrue != "true" {
		t.Errorf("Expected SuccessTrue to be 'true', got %s", SuccessTrue)
	}

	if SuccessFalse != "false" {
		t.Errorf("Expected SuccessFalse to be 'false', got %s", SuccessFalse)
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly registered with prometheus
	metricNames := []string{
		"lilrag_http_request_duration_seconds",
		"lilrag_http_requests_total",
		"lilrag_search_duration_seconds",
		"lilrag_search_requests_total",
		"lilrag_search_results_found",
		"lilrag_indexing_duration_seconds",
		"lilrag_indexing_requests_total",
		"lilrag_document_characters_indexed",
		"lilrag_chat_duration_seconds",
		"lilrag_chat_requests_total",
		"lilrag_chat_sources_retrieved",
		"lilrag_chat_response_length_characters",
		"lilrag_embedding_tokens_total",
		"lilrag_documents_total",
	}

	// Gather metrics from the default prometheus registry
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Create a map of registered metric names
	registeredMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		registeredMetrics[*mf.Name] = true
	}

	// Check that all our metrics are registered
	for _, metricName := range metricNames {
		if !registeredMetrics[metricName] {
			t.Errorf("Metric %s is not registered with prometheus", metricName)
		}
	}
}

func TestMetricsLabels(t *testing.T) {
	// Test that metrics with labels work correctly
	t.Run("http_request_labels", func(t *testing.T) {
		RecordHTTPRequest("GET", "/test", 200, 10*time.Millisecond)

		// Gather metrics
		metricFamilies, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics: %v", err)
		}

		// Look for our HTTP request metrics
		foundHTTPDuration := false
		foundHTTPTotal := false

		for _, mf := range metricFamilies {
			if *mf.Name == "lilrag_http_request_duration_seconds" {
				foundHTTPDuration = true
			}
			if *mf.Name == "lilrag_http_requests_total" {
				foundHTTPTotal = true
			}
		}

		if !foundHTTPDuration {
			t.Error("HTTP duration metric not found after recording")
		}
		if !foundHTTPTotal {
			t.Error("HTTP total metric not found after recording")
		}
	})

	t.Run("search_request_labels", func(t *testing.T) {
		RecordSearchRequest(50*time.Millisecond, true, 5)
		// Just verify it doesn't panic - detailed metric validation is complex
	})

	t.Run("chat_request_labels", func(t *testing.T) {
		RecordChatRequest(100*time.Millisecond, true, 3, 200)
		// Just verify it doesn't panic - detailed metric validation is complex
	})
}

func TestMetricsWithEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "negative duration handling",
			fn: func() {
				RecordSearchRequest(-time.Millisecond, true, 1)
			},
		},
		{
			name: "negative counts handling",
			fn: func() {
				RecordSearchRequest(time.Millisecond, true, -1)
				RecordChatRequest(time.Millisecond, true, -1, -1)
				RecordEmbeddingTokens("test-model", "test", false)
				UpdateDocumentCount(-1)
			},
		},
		{
			name: "empty strings handling",
			fn: func() {
				RecordHTTPRequest("", "", 0, 0)
				RecordEmbeddingTokens("", "", false)
			},
		},
		{
			name: "very large values handling",
			fn: func() {
				RecordSearchRequest(24*time.Hour, true, 1000000)
				RecordChatRequest(time.Hour, true, 100, 1000000)
				RecordEmbeddingTokens("large-model", string(make([]byte, 1000000)), false)
				UpdateDocumentCount(1000000)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should not panic even with edge case values
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Function panicked with edge case input: %v", r)
				}
			}()
			tt.fn()
		})
	}
}

func TestMetricsBuckets(t *testing.T) {
	// Test that histogram buckets are reasonable
	t.Run("search_buckets", func(t *testing.T) {
		expectedBuckets := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

		// We can't directly access the buckets from the metric, but we can verify they were set
		// by recording values and checking they don't panic
		for _, bucket := range expectedBuckets {
			RecordSearchRequest(time.Duration(bucket*float64(time.Second)), true, 1)
		}
	})

	t.Run("results_buckets", func(t *testing.T) {
		expectedBuckets := []float64{0, 1, 2, 5, 10, 20, 50, 100}

		for _, bucket := range expectedBuckets {
			RecordSearchRequest(time.Millisecond, true, int(bucket))
		}
	})
}

func TestConcurrentMetrics(t *testing.T) {
	// Test that metrics are thread-safe
	const numGoroutines = 10
	const operationsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < operationsPerGoroutine; j++ {
				RecordHTTPRequest("GET", "/test", 200, time.Millisecond)
				RecordSearchRequest(time.Millisecond, true, 1)
				RecordChatRequest(time.Millisecond, true, 1, 100)
				RecordIndexingRequest(time.Millisecond, true, 1000)
				RecordEmbeddingTokens("test-model", "test text", false)
				UpdateDocumentCount(id*operationsPerGoroutine + j)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func BenchmarkRecordHTTPRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordHTTPRequest("GET", "/api/test", 200, time.Millisecond)
	}
}

func BenchmarkRecordSearchRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordSearchRequest(time.Millisecond, true, 5)
	}
}

func BenchmarkRecordChatRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordChatRequest(100*time.Millisecond, true, 3, 200)
	}
}

func BenchmarkRecordIndexingRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordIndexingRequest(100*time.Millisecond, true, 1000)
	}
}

func BenchmarkUpdateDocumentCount(b *testing.B) {
	for i := 0; i < b.N; i++ {
		UpdateDocumentCount(i)
	}
}
