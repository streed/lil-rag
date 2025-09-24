# Search Scoring in Lil-RAG

## Overview

Lil-RAG uses **cosine distance** between query embeddings and chunk embeddings to score search results. This document explains how the search scoring system works.

## Implementation Details

### Core Algorithm

1. **Embedding Calculation**: Query text is converted to an embedding vector using the configured embedding model
2. **Distance Calculation**: For each indexed chunk, the system calculates the cosine distance between the query embedding and the chunk's embedding using SQLite's `vec_distance_cosine()` function
3. **Score Conversion**: The distance is converted to a score using: `score = 1.0 - distance`

### SQL Query

The search uses this SQL query to find and score results:

```sql
SELECT 
    c.document_id,
    c.chunk_text_compressed,
    c.chunk_index,
    c.page_number,
    c.chunk_type,
    d.original_text_compressed,
    d.file_path,
    d.source_path,
    vec_distance_cosine(e.embedding, ?) as distance
FROM chunks c
JOIN documents d ON c.document_id = d.id
JOIN embeddings e ON c.chunk_id = e.chunk_id
ORDER BY distance
LIMIT ?
```

Key points:
- **`e.embedding`** contains the chunk's embedding vector
- **`?`** is the query embedding vector
- **`vec_distance_cosine()`** calculates cosine distance between the two vectors

### Score Interpretation

| Distance | Score | Meaning |
|----------|-------|---------|
| 0.0 | 1.0 | Identical embeddings (perfect match) |
| 1.0 | 0.0 | Orthogonal embeddings (no similarity) |
| 2.0 | -1.0 | Opposite embeddings (maximum dissimilarity) |

- **Higher scores** indicate **better matches**
- **Scores close to 1.0** indicate very similar content
- **Scores around 0.0** indicate neutral/unrelated content
- **Negative scores** indicate semantically opposite content

### Chunk-Level vs Document-Level

The search system operates at the **chunk level**, not the document level:

- Each document is split into chunks during indexing
- Each chunk gets its own embedding
- Search compares the query against **individual chunk embeddings**
- This enables more precise matching of specific parts of documents

## Example

```go
// Query: "machine learning algorithms"
// Query embedding: [0.8, 0.2, 0.1, ...]

// Document chunks:
// Chunk 1: "Machine learning is a subset of AI" → embedding: [0.9, 0.1, 0.05, ...]
// Chunk 2: "The weather is sunny today" → embedding: [0.1, 0.8, 0.7, ...]

// Results:
// Chunk 1: distance=0.05, score=0.95 (very similar)
// Chunk 2: distance=1.2, score=-0.2 (dissimilar)
```

## Validation

The search scoring behavior is validated in the test `TestSQLiteStorage_SearchScoring_ChunkEmbeddingDistance`, which verifies:

- Identical embeddings get score ≈ 1.0
- Close embeddings get high scores (0.9+)
- Orthogonal embeddings get score ≈ 0.0
- Opposite embeddings get negative scores
- Results are sorted by score in descending order

## Performance

- The system uses SQLite's vector extension for efficient cosine distance calculations
- Results are pre-sorted by the database, minimizing post-processing
- The `LIMIT` clause ensures only the top results are processed