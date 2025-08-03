-- Create vector index for similarity search if vector extension is available
-- This is separated to allow the base schema to work without vector support

-- Try to create vector index (will fail silently if extension not available)
CREATE INDEX IF NOT EXISTS idx_code_chunks_embedding 
ON code_chunks(libsql_vector_idx(embedding, 'metric=cosine'));