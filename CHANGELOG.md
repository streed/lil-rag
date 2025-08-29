# Changelog

All notable changes to Lil-RAG will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of Lil-RAG
- CLI interface for indexing and searching documents
- HTTP API server with RESTful endpoints
- SQLite vector storage with sqlite-vec extension
- Ollama integration for embeddings
- PDF parsing with page-based chunking
- Automatic text compression using gzip
- Document deduplication in search results
- Profile-based configuration system
- File upload support for HTTP API
- Comprehensive examples and documentation

### Features
- 🔍 Semantic vector search with cosine similarity
- 📄 Document deduplication for multi-chunk documents
- 🗜️ Transparent gzip compression for storage optimization
- 📚 Native PDF support with page extraction
- 🔧 Dual CLI and HTTP API interfaces
- 🤖 Configurable Ollama embedding models
- ⚡ High-performance Go implementation
- 🎛️ User-friendly profile configuration
- 📁 Support for text files, PDFs, and stdin input
- 🔄 Complete document content in search results

### Technical Details
- Go 1.21+ with CGO support required
- SQLite with sqlite-vec extension for vector operations
- Automatic chunking for large documents
- Metadata preservation (page numbers, chunk indices)
- File path tracking for document locations
- Error handling with context wrapping
- Concurrent-safe database operations

## [1.0.0] - TBD

Initial public release.