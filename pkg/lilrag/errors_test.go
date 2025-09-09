package lilrag

import (
	"errors"
	"testing"
)

func TestLilRagError_Creation(t *testing.T) {
	tests := []struct {
		name         string
		errorFunc    func() *LilRagError
		expectedType ErrorType
		expectedCode string
	}{
		{
			name:         "validation error",
			errorFunc:    func() *LilRagError { return NewValidationError("test validation error", nil) },
			expectedType: ErrorTypeValidation,
			expectedCode: "VALIDATION_ERROR",
		},
		{
			name:         "config error",
			errorFunc:    func() *LilRagError { return NewConfigError("test config error", nil) },
			expectedType: ErrorTypeConfig,
			expectedCode: "CONFIG_ERROR",
		},
		{
			name:         "storage error",
			errorFunc:    func() *LilRagError { return NewStorageError("test storage error", nil) },
			expectedType: ErrorTypeStorage,
			expectedCode: "STORAGE_ERROR",
		},
		{
			name:         "embedding error",
			errorFunc:    func() *LilRagError { return NewEmbeddingError("test embedding error", nil) },
			expectedType: ErrorTypeEmbedding,
			expectedCode: "EMBEDDING_ERROR",
		},
		{
			name:         "parsing error",
			errorFunc:    func() *LilRagError { return NewParsingError("test parsing error", nil) },
			expectedType: ErrorTypeParsing,
			expectedCode: "PARSING_ERROR",
		},
		{
			name:         "search error",
			errorFunc:    func() *LilRagError { return NewSearchError("test search error", nil) },
			expectedType: ErrorTypeSearch,
			expectedCode: "SEARCH_ERROR",
		},
		{
			name:         "chat error",
			errorFunc:    func() *LilRagError { return NewChatError("test chat error", nil) },
			expectedType: ErrorTypeChat,
			expectedCode: "CHAT_ERROR",
		},
		{
			name:         "network error",
			errorFunc:    func() *LilRagError { return NewNetworkError("test network error", nil) },
			expectedType: ErrorTypeNetwork,
			expectedCode: "NETWORK_ERROR",
		},
		{
			name:         "internal error",
			errorFunc:    func() *LilRagError { return NewInternalError("test internal error", nil) },
			expectedType: ErrorTypeInternal,
			expectedCode: "INTERNAL_ERROR",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errorFunc()
			
			if err.Type != tt.expectedType {
				t.Errorf("Expected type %v, got %v", tt.expectedType, err.Type)
			}
			
			if err.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, err.Code)
			}
			
			if err.Message == "" {
				t.Error("Expected non-empty message")
			}
		})
	}
}

func TestLilRagError_WithCause(t *testing.T) {
	originalErr := errors.New("original error")
	lilragErr := NewStorageError("storage failed", originalErr)
	
	if lilragErr.Cause != originalErr {
		t.Errorf("Expected cause to be %v, got %v", originalErr, lilragErr.Cause)
	}
	
	errorStr := lilragErr.Error()
	if !contains(errorStr, "STORAGE_ERROR") {
		t.Error("Error string should contain error code")
	}
	if !contains(errorStr, "storage failed") {
		t.Error("Error string should contain message")
	}
	if !contains(errorStr, "original error") {
		t.Error("Error string should contain cause")
	}
}

func TestLilRagError_WithContext(t *testing.T) {
	err := NewValidationError("validation failed", nil)
	
	// Add context
	err.WithContext("field", "email")
	err.WithContext("value", "invalid@")
	
	if err.Context == nil {
		t.Fatal("Expected context to be initialized")
	}
	
	if err.Context["field"] != "email" {
		t.Errorf("Expected field context to be 'email', got %v", err.Context["field"])
	}
	
	if err.Context["value"] != "invalid@" {
		t.Errorf("Expected value context to be 'invalid@', got %v", err.Context["value"])
	}
}

func TestLilRagError_WithOperation(t *testing.T) {
	err := NewEmbeddingError("embedding failed", nil)
	err.WithOperation("IndexDocument")
	
	if err.Operation != "IndexDocument" {
		t.Errorf("Expected operation to be 'IndexDocument', got %s", err.Operation)
	}
}

func TestLilRagError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	lilragErr := NewChatError("chat failed", originalErr)
	
	unwrapped := lilragErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Expected unwrapped error to be %v, got %v", originalErr, unwrapped)
	}
}

func TestLilRagError_Is(t *testing.T) {
	tests := []struct {
		name           string
		err            *LilRagError
		target         error
		shouldMatch    bool
	}{
		{
			name:        "config error matches ErrInvalidConfig",
			err:         NewConfigError("invalid config", nil),
			target:      ErrInvalidConfig,
			shouldMatch: true,
		},
		{
			name:        "storage error matches ErrDocumentNotFound",
			err:         NewStorageError("document not found", nil),
			target:      ErrDocumentNotFound,
			shouldMatch: true,
		},
		{
			name:        "embedding error matches ErrEmbeddingFailed",
			err:         NewEmbeddingError("embedding failed", nil),
			target:      ErrEmbeddingFailed,
			shouldMatch: true,
		},
		{
			name:        "parsing error matches ErrUnsupportedFormat",
			err:         NewParsingError("unsupported format", nil),
			target:      ErrUnsupportedFormat,
			shouldMatch: true,
		},
		{
			name:        "search error matches ErrSearchFailed",
			err:         NewSearchError("search failed", nil),
			target:      ErrSearchFailed,
			shouldMatch: true,
		},
		{
			name:        "chat error matches ErrChatFailed",
			err:         NewChatError("chat failed", nil),
			target:      ErrChatFailed,
			shouldMatch: true,
		},
		{
			name:        "storage error doesn't match embedding error",
			err:         NewStorageError("storage failed", nil),
			target:      ErrEmbeddingFailed,
			shouldMatch: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := errors.Is(tt.err, tt.target)
			if matches != tt.shouldMatch {
				t.Errorf("Expected Is(%v) to be %v, got %v", tt.target, tt.shouldMatch, matches)
			}
		})
	}
}

func TestLilRagError_IsWithCause(t *testing.T) {
	// Test that Is works with wrapped causes
	originalErr := errors.New("connection refused")
	lilragErr := NewNetworkError("network failed", originalErr)
	
	// Should match the original error through cause
	if !errors.Is(lilragErr, originalErr) {
		t.Error("Expected error to match original cause")
	}
}

func TestValidateDocumentID(t *testing.T) {
	tests := []struct {
		name        string
		documentID  string
		expectError bool
	}{
		{
			name:       "valid document ID",
			documentID: "doc123",
		},
		{
			name:        "empty document ID",
			documentID:  "",
			expectError: true,
		},
		{
			name:        "too long document ID",
			documentID:  string(make([]byte, 256)), // 256 characters
			expectError: true,
		},
		{
			name:       "max length document ID",
			documentID: string(make([]byte, 255)), // 255 characters
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDocumentID(tt.documentID)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if err != nil {
				// Should be a validation error
				var lilragErr *LilRagError
				if !errors.As(err, &lilragErr) {
					t.Error("Expected LilRagError")
				} else if lilragErr.Type != ErrorTypeValidation {
					t.Errorf("Expected validation error, got %v", lilragErr.Type)
				}
			}
		})
	}
}

func TestValidateSearchQuery(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectError bool
	}{
		{
			name:  "valid query",
			query: "test search query",
		},
		{
			name:        "empty query",
			query:       "",
			expectError: true,
		},
		{
			name:        "too long query",
			query:       string(make([]byte, 10001)), // 10001 characters
			expectError: true,
		},
		{
			name:  "max length query",
			query: string(make([]byte, 10000)), // 10000 characters
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSearchQuery(tt.query)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSearchLimit(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		expectError bool
	}{
		{
			name:  "valid limit",
			limit: 10,
		},
		{
			name:  "max limit",
			limit: 100,
		},
		{
			name:        "zero limit",
			limit:       0,
			expectError: true,
		},
		{
			name:        "negative limit",
			limit:       -1,
			expectError: true,
		},
		{
			name:        "too high limit",
			limit:       101,
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSearchLimit(tt.limit)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidateText(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		expectError bool
	}{
		{
			name: "valid text",
			text: "This is valid text content",
		},
		{
			name:        "empty text",
			text:        "",
			expectError: true,
		},
		{
			name:        "too large text",
			text:        string(make([]byte, 11*1024*1024)), // 11MB
			expectError: true,
		},
		{
			name: "max size text",
			text: string(make([]byte, 10*1024*1024)), // 10MB
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateText(tt.text)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestToErrorResponse(t *testing.T) {
	t.Run("LilRagError conversion", func(t *testing.T) {
		originalErr := NewValidationError("validation failed", nil)
		originalErr.WithContext("field", "email").WithOperation("CreateUser")
		
		response := ToErrorResponse(originalErr)
		
		if response.Type != string(ErrorTypeValidation) {
			t.Errorf("Expected type %s, got %s", ErrorTypeValidation, response.Type)
		}
		if response.Code != "VALIDATION_ERROR" {
			t.Errorf("Expected code VALIDATION_ERROR, got %s", response.Code)
		}
		if response.Message != "validation failed" {
			t.Errorf("Expected message 'validation failed', got %s", response.Message)
		}
		if response.Context["field"] != "email" {
			t.Error("Expected context to be preserved")
		}
	})
	
	t.Run("regular error conversion", func(t *testing.T) {
		regularErr := errors.New("regular error")
		response := ToErrorResponse(regularErr)
		
		if response.Type != string(ErrorTypeInternal) {
			t.Errorf("Expected type %s, got %s", ErrorTypeInternal, response.Type)
		}
		if response.Code != "UNKNOWN_ERROR" {
			t.Errorf("Expected code UNKNOWN_ERROR, got %s", response.Code)
		}
		if !contains(response.Error, "regular error") {
			t.Error("Expected error message to contain original error")
		}
	})
}

func TestErrorChaining(t *testing.T) {
	// Test complex error chaining
	originalErr := errors.New("database connection failed")
	storageErr := NewStorageError("failed to store document", originalErr)
	embeddingErr := NewEmbeddingError("failed to generate embedding", storageErr)
	
	// Should be able to unwrap through the chain
	if !errors.Is(embeddingErr, originalErr) {
		t.Error("Expected to find original error in chain")
	}
	
	if !errors.Is(embeddingErr, storageErr) {
		t.Error("Expected to find storage error in chain")
	}
	
	// Check error message contains all levels
	errorStr := embeddingErr.Error()
	if !contains(errorStr, "EMBEDDING_ERROR") {
		t.Error("Expected embedding error code")
	}
	if !contains(errorStr, "failed to generate embedding") {
		t.Error("Expected embedding error message")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	if start+len(substr) > len(s) {
		return false
	}
	for i := 0; i < len(substr); i++ {
		if s[start+i] != substr[i] {
			if start+1 < len(s) {
				return containsAt(s, substr, start+1)
			}
			return false
		}
	}
	return true
}