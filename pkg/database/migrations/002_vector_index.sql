-- Create HNSW vector index for similarity search with DuckDB VSS extension
-- This will only work if the VSS extension is loaded

-- Try to create HNSW index for vector similarity search
-- This uses DuckDB's VSS extension if available
-- The index will significantly speed up vector similarity queries

-- Note: This will fail silently if VSS extension is not available
-- The application will fall back to non-indexed vector search or keyword search

-- Create HNSW index on the embedding column
-- Using cosine distance metric for similarity
-- CREATE INDEX IF NOT EXISTS idx_code_chunks_embedding 
-- ON code_chunks USING HNSW (embedding) 
-- WITH (metric = 'cosine');

-- For now, we'll skip the HNSW index creation as it requires
-- special handling in DuckDB and will be created programmatically
-- when the VSS extension is available