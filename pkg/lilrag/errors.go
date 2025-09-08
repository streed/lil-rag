package lilrag

import (
	"errors"
	"fmt"
)

// Error types for better error handling and categorization
var (
	// Configuration errors
	ErrInvalidConfig    = errors.New("invalid configuration")
	ErrMissingConfig    = errors.New("missing required configuration")
	ErrConfigValidation = errors.New("configuration validation failed")

	// Storage errors
	ErrStorageInit       = errors.New("storage initialization failed")
	ErrStorageUnavail    = errors.New("storage unavailable")
	ErrDocumentNotFound  = errors.New("document not found")
	ErrChunkNotFound     = errors.New("chunk not found")
	ErrDuplicateDocument = errors.New("document already exists")

	// Embedding errors
	ErrEmbeddingFailed  = errors.New("embedding generation failed")
	ErrModelUnavailable = errors.New("embedding model unavailable")
	ErrEmbeddingTimeout = errors.New("embedding request timeout")
	ErrInvalidVector    = errors.New("invalid vector data")

	// Parsing errors
	ErrUnsupportedFormat = errors.New("unsupported file format")
	ErrParsingFailed     = errors.New("document parsing failed")
	ErrEmptyContent      = errors.New("no content found in document")
	ErrCorruptedFile     = errors.New("file appears to be corrupted")

	// Search errors
	ErrSearchFailed = errors.New("search operation failed")
	ErrEmptyQuery   = errors.New("search query cannot be empty")
	ErrInvalidLimit = errors.New("invalid search limit")

	// Chat errors
	ErrChatFailed      = errors.New("chat operation failed")
	ErrChatUnavailable = errors.New("chat service unavailable")
	ErrEmptyMessage    = errors.New("message cannot be empty")
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeConfig     ErrorType = "configuration"
	ErrorTypeStorage    ErrorType = "storage"
	ErrorTypeEmbedding  ErrorType = "embedding"
	ErrorTypeParsing    ErrorType = "parsing"
	ErrorTypeSearch     ErrorType = "search"
	ErrorTypeChat       ErrorType = "chat"
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeInternal   ErrorType = "internal"
)

// LilRagError provides structured error information with context
type LilRagError struct {
	Type      ErrorType
	Code      string
	Message   string
	Cause     error
	Context   map[string]interface{}
	Operation string
}

// Error implements the error interface
func (e *LilRagError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *LilRagError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a target error
func (e *LilRagError) Is(target error) bool {
	if e.Cause != nil && errors.Is(e.Cause, target) {
		return true
	}

	// Check against predefined errors
	switch target {
	case ErrInvalidConfig, ErrMissingConfig, ErrConfigValidation:
		return e.Type == ErrorTypeConfig
	case ErrStorageInit, ErrStorageUnavail, ErrDocumentNotFound, ErrChunkNotFound:
		return e.Type == ErrorTypeStorage
	case ErrEmbeddingFailed, ErrModelUnavailable, ErrEmbeddingTimeout:
		return e.Type == ErrorTypeEmbedding
	case ErrUnsupportedFormat, ErrParsingFailed, ErrEmptyContent:
		return e.Type == ErrorTypeParsing
	case ErrSearchFailed, ErrEmptyQuery:
		return e.Type == ErrorTypeSearch
	case ErrChatFailed, ErrChatUnavailable:
		return e.Type == ErrorTypeChat
	}

	return false
}

// WithContext adds context to the error
func (e *LilRagError) WithContext(key string, value interface{}) *LilRagError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithOperation sets the operation context
func (e *LilRagError) WithOperation(operation string) *LilRagError {
	e.Operation = operation
	return e
}

// Error creation helper functions

// NewValidationError creates a validation error
func NewValidationError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeValidation,
		Code:    "VALIDATION_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewConfigError creates a configuration error
func NewConfigError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeConfig,
		Code:    "CONFIG_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewStorageError creates a storage error
func NewStorageError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeStorage,
		Code:    "STORAGE_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewEmbeddingError creates an embedding error
func NewEmbeddingError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeEmbedding,
		Code:    "EMBEDDING_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewParsingError creates a parsing error
func NewParsingError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeParsing,
		Code:    "PARSING_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewSearchError creates a search error
func NewSearchError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeSearch,
		Code:    "SEARCH_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewChatError creates a chat error
func NewChatError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeChat,
		Code:    "CHAT_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewNetworkError creates a network error
func NewNetworkError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeNetwork,
		Code:    "NETWORK_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// NewInternalError creates an internal error
func NewInternalError(message string, cause error) *LilRagError {
	return &LilRagError{
		Type:    ErrorTypeInternal,
		Code:    "INTERNAL_ERROR",
		Message: message,
		Cause:   cause,
	}
}

// Validation helper functions

// ValidateDocumentID checks if a document ID is valid
func ValidateDocumentID(id string) error {
	if id == "" {
		return NewValidationError("document ID cannot be empty", nil)
	}
	if len(id) > 255 {
		return NewValidationError("document ID too long (max 255 characters)", nil)
	}
	return nil
}

// ValidateSearchQuery checks if a search query is valid
func ValidateSearchQuery(query string) error {
	if query == "" {
		return NewValidationError("search query cannot be empty", nil)
	}
	if len(query) > 10000 {
		return NewValidationError("search query too long (max 10000 characters)", nil)
	}
	return nil
}

// ValidateSearchLimit checks if a search limit is valid
func ValidateSearchLimit(limit int) error {
	if limit < 1 {
		return NewValidationError("search limit must be positive", nil)
	}
	if limit > 100 {
		return NewValidationError("search limit too high (max 100)", nil)
	}
	return nil
}

// ValidateText checks if text content is valid for processing
func ValidateText(text string) error {
	if text == "" {
		return NewValidationError("text content cannot be empty", nil)
	}
	if len(text) > 10*1024*1024 { // 10MB limit
		return NewValidationError("text content too large (max 10MB)", nil)
	}
	return nil
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Type    string                 `json:"type,omitempty"`
	Code    string                 `json:"code,omitempty"`
	Message string                 `json:"message,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// ToErrorResponse converts a LilRagError to an API error response
func ToErrorResponse(err error) ErrorResponse {
	var lilragErr *LilRagError
	if errors.As(err, &lilragErr) {
		return ErrorResponse{
			Error:   lilragErr.Error(),
			Type:    string(lilragErr.Type),
			Code:    lilragErr.Code,
			Message: lilragErr.Message,
			Context: lilragErr.Context,
		}
	}

	return ErrorResponse{
		Error:   err.Error(),
		Type:    string(ErrorTypeInternal),
		Code:    "UNKNOWN_ERROR",
		Message: "An unexpected error occurred",
	}
}
