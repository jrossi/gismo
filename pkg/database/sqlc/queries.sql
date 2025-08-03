-- name: InsertProject :one
INSERT INTO projects (
    project_name, project_path, description
) VALUES (
    ?, ?, ?
) ON CONFLICT(project_name) DO UPDATE SET
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetProjectByName :one
SELECT * FROM projects WHERE project_name = ?;

-- name: GetProjectByPath :one
SELECT * FROM projects WHERE project_path = ?;

-- name: UpdateProjectIndexTime :exec
UPDATE projects 
SET last_indexed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: InsertCodeChunk :one
INSERT INTO code_chunks (
    project_id, file_path, absolute_path, content, chunk_type, language, 
    start_line, end_line, embedding, metadata
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, vector(?), ?
) RETURNING *;

-- name: InsertCodeChunkWithoutEmbedding :one
INSERT INTO code_chunks (
    project_id, file_path, absolute_path, content, chunk_type, language, 
    start_line, end_line, metadata
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: UpdateCodeChunkEmbedding :exec
UPDATE code_chunks 
SET embedding = vector(?), updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetCodeChunkByID :one
SELECT * FROM code_chunks WHERE id = ?;

-- name: GetCodeChunksByFilePath :many
SELECT * FROM code_chunks 
WHERE project_id = ? AND file_path = ? 
ORDER BY start_line;

-- name: DeleteCodeChunksByFilePath :exec
DELETE FROM code_chunks WHERE project_id = ? AND file_path = ?;

-- name: DeleteCodeChunksByProject :exec
DELETE FROM code_chunks WHERE project_id = ?;

-- name: SearchCodeChunksByLanguage :many
SELECT cc.*, p.project_name, p.project_path 
FROM code_chunks cc
JOIN projects p ON cc.project_id = p.id
WHERE cc.project_id = ? AND cc.language = ? 
ORDER BY cc.file_path, cc.start_line
LIMIT ?;

-- name: SearchCodeChunksByType :many
SELECT cc.*, p.project_name, p.project_path 
FROM code_chunks cc
JOIN projects p ON cc.project_id = p.id
WHERE cc.project_id = ? AND cc.chunk_type = ? 
ORDER BY cc.file_path, cc.start_line
LIMIT ?;

-- name: GetTotalChunksCount :one
SELECT COUNT(*) as count FROM code_chunks WHERE project_id = ?;

-- name: GetLanguageStats :many
SELECT language, COUNT(*) as count 
FROM code_chunks 
WHERE project_id = ?
GROUP BY language 
ORDER BY count DESC;

-- name: GetChunkTypeStats :many
SELECT chunk_type, COUNT(*) as count 
FROM code_chunks 
WHERE project_id = ?
GROUP BY chunk_type 
ORDER BY count DESC;

-- name: GetFileCount :one
SELECT COUNT(DISTINCT file_path) as count FROM code_chunks WHERE project_id = ?;

-- name: GetAllProjects :many
SELECT * FROM projects 
ORDER BY project_name;

-- name: InsertSearchHistory :one
INSERT INTO search_history (
    query, result_count, filters, execution_time_ms
) VALUES (
    ?, ?, ?, ?
) RETURNING *;

-- name: GetRecentSearches :many
SELECT * FROM search_history 
ORDER BY created_at DESC 
LIMIT ?;

-- name: UpdateIndexStats :one
INSERT INTO index_stats (
    total_chunks, total_files, languages, last_indexed_at
) VALUES (
    ?, ?, ?, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetLatestIndexStats :one
SELECT * FROM index_stats 
ORDER BY created_at DESC 
LIMIT 1;