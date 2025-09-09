# Mini-RAG Comprehensive Test Plan

## Overview
This document outlines a comprehensive testing strategy for the mini-rag project, covering all major components and functionality.

## Current Test Coverage Analysis

### Coverage Summary (as of analysis)
- **pkg/lilrag**: 25.9% coverage
- **internal/handlers**: 15.0% coverage  
- **pkg/config**: 33.7% coverage
- **pkg/metrics**: 0.0% coverage
- **cmd/***: 0.0% coverage
- **internal/theme**: 0.0% coverage

## Core Functionality Categories

### 1. Document Processing & Parsing
**Components**: Document parsers, chunking, content extraction
- [x] Text chunker (chunker_test.go) - **COVERED**
- [x] Compression (compression_test.go) - **COVERED** 
- [ ] **MISSING**: PDF parser tests
- [ ] **MISSING**: Image parser (OCR) tests
- [ ] **MISSING**: DOCX parser tests
- [ ] **MISSING**: CSV parser tests
- [ ] **MISSING**: XLSX parser tests
- [ ] **MISSING**: HTML parser tests
- [ ] **MISSING**: Document handler tests

### 2. Storage & Retrieval
**Components**: SQLite storage, vector operations, CRUD
- [x] SQLite storage (storage_test.go) - **COVERED**
- [x] Basic CRUD operations - **COVERED**
- [ ] **MISSING**: Namespace functionality tests
- [ ] **MISSING**: Migration tests
- [ ] **MISSING**: Concurrent access tests
- [ ] **MISSING**: Data integrity tests

### 3. Embedding & AI Integration  
**Components**: Ollama integration, embeddings, chat
- [x] Ollama embedder (embedder_test.go) - **COVERED**
- [ ] **MISSING**: Chat functionality tests
- [ ] **MISSING**: Vision model tests
- [ ] **MISSING**: Embedding cache tests
- [ ] **MISSING**: Model fallback tests

### 4. HTTP API & Handlers
**Components**: REST API, file uploads, WebUI
- [x] Basic API handlers (handlers_test.go) - **PARTIAL**
- [ ] **MISSING**: File upload edge cases
- [ ] **MISSING**: Authentication/security tests
- [ ] **MISSING**: Rate limiting tests
- [ ] **MISSING**: Error response format validation
- [ ] **MISSING**: Multipart form handling

### 5. Configuration & Setup
**Components**: Config loading, profiles, validation
- [x] Config loading (config_test.go) - **COVERED**
- [ ] **MISSING**: Profile management tests
- [ ] **MISSING**: Environment variable handling
- [ ] **MISSING**: Configuration validation

### 6. Service Layer Architecture
**Components**: New service abstraction, factory patterns
- [ ] **MISSING**: Service factory tests  
- [ ] **MISSING**: Service interface implementations
- [ ] **MISSING**: Dependency injection tests
- [ ] **MISSING**: Service composition tests

### 7. Error Handling & Resilience
**Components**: Structured errors, retry logic, fallbacks
- [ ] **MISSING**: Error categorization tests
- [ ] **MISSING**: Error context preservation  
- [ ] **MISSING**: Retry mechanism tests
- [ ] **MISSING**: Circuit breaker tests

## Testing Strategy by Type

### Unit Tests (Isolated Component Testing)
**Priority**: High | **Current Coverage**: ~20%
- Individual function/method testing
- Mock dependencies for external services
- Fast execution (<100ms per test)
- High code coverage target: 80%+

### Integration Tests (Component Interaction)
**Priority**: High | **Current Coverage**: Minimal
- Database + Storage interactions
- API + Service layer integration
- File processing pipelines
- Ollama service integration

### End-to-End Tests (Full Workflow)
**Priority**: Medium | **Current Coverage**: None
- Complete document indexing flows
- Search and retrieval workflows
- API client interactions
- WebUI functionality

### Performance Tests (Load & Stress)
**Priority**: Medium | **Current Coverage**: None
- Concurrent document processing
- Large file handling
- Memory usage profiling
- Response time benchmarks

### Security Tests (Vulnerability Assessment)
**Priority**: Medium | **Current Coverage**: None
- Input validation and sanitization
- File upload security
- SQL injection prevention
- XSS protection

## Test Implementation Plan

### Phase 1: Critical Unit Tests (Week 1)
1. **Document Parsers**: Add tests for all parser types
2. **Error Framework**: Test new error handling system
3. **Service Layer**: Test factory and service interfaces
4. **Namespace Support**: Test new namespace functionality

### Phase 2: Integration Coverage (Week 2)
1. **API Integration**: Complete handler test coverage
2. **Storage Integration**: Multi-component storage tests
3. **Pipeline Tests**: End-to-end document processing
4. **Configuration Tests**: Profile and environment handling

### Phase 3: Advanced Testing (Week 3)
1. **Performance Tests**: Load testing and benchmarks
2. **Concurrency Tests**: Thread safety and race conditions
3. **Error Scenarios**: Failure mode testing
4. **Edge Cases**: Boundary condition testing

### Phase 4: Automation & CI (Week 4)
1. **Test Automation**: CI/CD pipeline integration
2. **Coverage Monitoring**: Automated coverage reporting
3. **Regression Suite**: Automated regression testing
4. **Documentation**: Test documentation and guidelines

## Test Data Requirements

### Sample Documents
- [ ] PDF files (various sizes, with/without images)
- [ ] Image files (PNG, JPG) for OCR testing
- [ ] DOCX documents with complex formatting
- [ ] CSV files with various data types
- [ ] XLSX spreadsheets with multiple sheets
- [ ] HTML files with embedded content
- [ ] Large files (>10MB) for performance testing
- [ ] Corrupted/invalid files for error testing

### Mock Services
- [ ] Mock Ollama server for consistent testing
- [ ] Mock embedding responses
- [ ] Mock chat completions
- [ ] Network failure simulations

## Acceptance Criteria

### Test Coverage Targets
- **Overall Coverage**: 80%+
- **Core Library (pkg/lilrag)**: 90%+
- **API Handlers**: 85%+
- **Configuration**: 90%+
- **Critical Paths**: 95%+

### Performance Benchmarks
- **Document Indexing**: <500ms per document (avg)
- **Search Response**: <100ms for basic queries
- **Concurrent Users**: Support 10+ concurrent operations
- **Memory Usage**: <500MB for normal operations

### Quality Gates
- [ ] All tests pass consistently
- [ ] No flaky tests (>99% reliability)
- [ ] Coverage metrics maintained
- [ ] Performance benchmarks met
- [ ] Security tests pass

## Tools and Frameworks

### Testing Tools
- **Go Testing**: Built-in testing framework
- **Testify**: Assertions and mocking
- **GoMock**: Interface mocking
- **httptest**: HTTP testing utilities

### Coverage and Reporting
- **go cover**: Built-in coverage analysis
- **gocov**: Coverage reporting
- **golangci-lint**: Static analysis

### Performance Testing
- **go test -bench**: Benchmarking
- **pprof**: Profiling and analysis
- **vegeta**: Load testing tool

## Continuous Integration

### GitHub Actions Workflow
```yaml
name: Test Suite
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - name: Run Tests
        run: go test -race -coverprofile=coverage.out ./...
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
```

### Test Environment Requirements
- **Ollama Service**: Mock or real instance for AI tests
- **SQLite**: In-memory databases for fast testing
- **Test Data**: Standardized test document set
- **Isolated Environment**: No shared state between tests

## Maintenance and Updates

### Test Maintenance Schedule
- **Weekly**: Run full test suite
- **Monthly**: Review and update test coverage
- **Per Release**: Performance benchmark validation
- **Continuous**: Automated regression testing

### Documentation Updates
- Keep test plan current with feature changes
- Document new test patterns and utilities
- Maintain test data and mock service setups
- Update coverage targets as code grows

---

## Next Steps

1. **Immediate**: Fix existing test failures
2. **Short-term**: Implement Phase 1 critical tests  
3. **Medium-term**: Build integration test suite
4. **Long-term**: Full CI/CD automation with quality gates

This comprehensive test plan provides a structured approach to achieving high-quality, well-tested code across all components of the mini-rag system.