# Exa.ai Integration

Gismo provides integrated support for Exa.ai web search with intelligent semantic caching, allowing Claude Code to
access up-to-date web information while minimizing API costs through smart caching.

## Features

### Semantic Caching
- **Smart Query Matching**: Uses embeddings to find similar previous searches
- **Configurable Similarity Threshold**: Control how similar queries need to be for cache hits
- **Project Isolation**: Each project maintains its own search cache

### Feedback-Driven TTL
- **Adaptive Cache Lifetime**: Cache entries start with 7-day TTL
- **Usefulness Scoring**: Provide feedback on search result quality
- **TTL Extension**: Useful results get extended TTL (up to 14 days)
- **Automatic Eviction**: Low-value results expire naturally

### Local Embeddings
- **Privacy-First**: Uses fastembed for local embedding generation
- **No Internet Required**: BGESmallEN model runs entirely locally
- **Fast Similarity Search**: DuckDB's vector operations for efficient queries

## Configuration

Add your Exa API key to your environment:

```bash
export EXA_API_KEY="your-api-key-here"
```

Configure in `.claude/gismo.json`:

```json
{
  "exa": {
    "enabled": true,
    "api_key": "${EXA_API_KEY}",
    "cache": {
      "enabled": true,
      "similarity_threshold": 0.8,
      "default_ttl_days": 7,
      "max_ttl_days": 30
    },
    "defaults": {
      "search_type": "neural",
      "num_results": 10,
      "use_autoprompt": true
    }
  }
}
```

## CLI Usage

### Search the Web

```bash
# Basic search
gismo-knowledge exa "kubernetes deployment strategies"

# Search with options
gismo-knowledge exa "machine learning papers" --type neural --num-results 20

# Search without cache
gismo-knowledge exa "latest news" --no-cache

# Custom similarity threshold
gismo-knowledge exa "distributed systems" --similarity 0.9
```

### View Cached Searches

```bash
# List all cached searches
gismo-knowledge exa-cached

# Filter by query
gismo-knowledge exa-cached --filter "kubernetes"

# Limit results
gismo-knowledge exa-cached --limit 20
```

### Provide Feedback

```bash
# Mark search as useful (extends TTL)
gismo-knowledge exa-feedback <search-id> --score 0.9

# Mark as not useful (reduces TTL)
gismo-knowledge exa-feedback <search-id> --score 0.2
```

## SQL Queries

Access the cache directly via SQL:

```sql
-- View all cached searches
SELECT id, query, search_type, access_count, ttl_days
FROM exa_search_cache
ORDER BY last_accessed DESC;

-- Find searches about a topic
SELECT query, created_at
FROM exa_search_cache
WHERE query LIKE '%kubernetes%';

-- View feedback history
SELECT s.query, f.usefulness_score, f.created_at
FROM exa_feedback f
JOIN exa_search_cache s ON f.search_id = s.id
ORDER BY f.created_at DESC;

-- Searches with high usefulness
SELECT query, AVG(usefulness_score) as avg_score
FROM exa_search_cache s
JOIN exa_feedback f ON s.id = f.search_id
GROUP BY query
HAVING avg_score > 0.7;
```

## gRPC API

The Exa.ai integration is exposed via gRPC:

### ExaSearch RPC
```protobuf
rpc ExaSearch(ExaSearchRequest) returns (ExaSearchResponse);
```

Performs a web search with caching support.

### ProvideFeedback RPC
```protobuf
rpc ProvideFeedback(SearchFeedbackRequest) returns (SearchFeedbackResponse);
```

Records feedback about search usefulness.

### GetCachedSearches RPC
```protobuf
rpc GetCachedSearches(GetCachedSearchesRequest) returns (GetCachedSearchesResponse);
```

Retrieves cached searches for analysis.

## Architecture

### Database Schema

The integration uses three main tables:

1. **exa_search_cache**: Stores search queries and results
   - Includes embeddings for semantic similarity
   - Tracks access patterns and TTL
   - Stores results as JSON for flexibility

2. **exa_feedback**: Records usefulness feedback
   - Links to search cache entries
   - Supports multiple feedback types
   - Influences TTL extensions

3. **exa_search_results**: Individual result storage (optional)
   - Currently unused (results stored as JSON)
   - Reserved for future detailed result tracking

### Cache Manager

The `ExaCacheManager` handles:
- Embedding generation via fastembed
- Similarity search using cosine distance
- TTL management based on feedback
- Project context isolation
- Automatic cache eviction

### Performance Considerations

- **Embedding Generation**: ~50ms for typical queries
- **Similarity Search**: <10ms for thousands of entries
- **Cache Hit Rate**: Typically 30-50% with 0.8 threshold
- **Storage**: ~2KB per cached search with results

## Security

- **API Key Protection**: Never logged or exposed
- **Project Isolation**: Searches cached per project
- **No PII in Cache**: Results sanitized before storage
- **Local Embeddings**: No data sent to external services

## Troubleshooting

### Empty Query Results

If `SELECT *` returns no results but `COUNT(*)` shows rows, ensure the server has the timestamp serialization fix applied.

### Embedding Initialization

If embeddings show as `<nil>`, check fastembed initialization:
```bash
ls ~/.cache/tokenizer  # Should contain model files
```

### Cache Not Working

Verify configuration:
```bash
gismo-knowledge exa "test" --debug  # Shows cache lookup details
```

## Future Enhancements

- **Semantic deduplication** of similar results
- **Cross-project knowledge sharing** (opt-in)
- **Result summarization** using local LLMs
- **Automatic query expansion** for better coverage
- **Cache warming** from common queries