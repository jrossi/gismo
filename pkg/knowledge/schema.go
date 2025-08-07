package knowledge

const createSchema = `
-- Create sequences
CREATE SEQUENCE IF NOT EXISTS docset_content_seq;
CREATE SEQUENCE IF NOT EXISTS exa_feedback_seq;
CREATE SEQUENCE IF NOT EXISTS exa_result_seq;
CREATE SEQUENCE IF NOT EXISTS exa_research_task_seq;
CREATE SEQUENCE IF NOT EXISTS exa_research_event_seq;

-- Docsets registry
CREATE TABLE IF NOT EXISTS docsets (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version TEXT,
  language TEXT,
  source_url TEXT,
  source_type TEXT, -- 'official', 'user', 'custom'
  imported_at TIMESTAMP DEFAULT NOW(),
  metadata JSON
);

-- Docset content with embeddings
CREATE TABLE IF NOT EXISTS docset_content (
  id INTEGER PRIMARY KEY DEFAULT nextval('docset_content_seq'),
  docset_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  path TEXT NOT NULL,
  content TEXT,
  summary TEXT,
  embedding REAL[],
  metadata JSON,
  FOREIGN KEY (docset_id) REFERENCES docsets(id)
);

-- Exa search cache with embeddings
CREATE TABLE IF NOT EXISTS exa_search_cache (
  id TEXT PRIMARY KEY,
  query TEXT NOT NULL,
  query_embedding REAL[],
  search_type TEXT,
  results JSON,
  created_at TIMESTAMP DEFAULT NOW(),
  last_accessed TIMESTAMP DEFAULT NOW(),
  access_count INTEGER DEFAULT 1,
  ttl_days INTEGER DEFAULT 7,
  project_context TEXT
);

-- Feedback tracking for cache intelligence
CREATE TABLE IF NOT EXISTS exa_feedback (
  id INTEGER PRIMARY KEY DEFAULT nextval('exa_feedback_seq'),
  search_id TEXT NOT NULL,
  usefulness_score FLOAT,
  feedback_type TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  FOREIGN KEY (search_id) REFERENCES exa_search_cache(id)
);

-- Individual search results with embeddings for granular search
CREATE TABLE IF NOT EXISTS exa_search_results (
  id INTEGER PRIMARY KEY DEFAULT nextval('exa_result_seq'),
  search_id TEXT NOT NULL,
  url TEXT NOT NULL,
  title TEXT,
  snippet TEXT,
  content TEXT,
  content_embedding REAL[],
  metadata JSON,
  FOREIGN KEY (search_id) REFERENCES exa_search_cache(id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_docset_content_name ON docset_content(name);
CREATE INDEX IF NOT EXISTS idx_docset_content_type ON docset_content(docset_id, type);
CREATE INDEX IF NOT EXISTS idx_exa_cache_query ON exa_search_cache(query);
CREATE INDEX IF NOT EXISTS idx_exa_cache_project ON exa_search_cache(project_context);
CREATE INDEX IF NOT EXISTS idx_exa_cache_accessed ON exa_search_cache(last_accessed);
CREATE INDEX IF NOT EXISTS idx_exa_results_search ON exa_search_results(search_id);

-- Exa Research task management
CREATE TABLE IF NOT EXISTS exa_research_tasks (
  id TEXT PRIMARY KEY DEFAULT 'ert_' || nextval('exa_research_task_seq')::TEXT,
  exa_task_id TEXT UNIQUE,
  instructions TEXT NOT NULL,
  model TEXT DEFAULT 'exa-research',
  output_schema JSON,
  status TEXT DEFAULT 'pending',
  progress_message TEXT,
  result JSON,
  citations JSON,
  error_message TEXT,
  estimated_cost REAL,
  actual_cost REAL,
  created_at TIMESTAMP DEFAULT NOW(),
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  project_context TEXT,
  user_consent BOOLEAN DEFAULT FALSE,
  consent_timestamp TIMESTAMP,
  retry_count INTEGER DEFAULT 0,
  max_retries INTEGER DEFAULT 3
);

-- Exa Research task events for audit trail
CREATE TABLE IF NOT EXISTS exa_research_events (
  id INTEGER PRIMARY KEY DEFAULT nextval('exa_research_event_seq'),
  task_id TEXT NOT NULL REFERENCES exa_research_tasks(id),
  event_type TEXT NOT NULL,
  event_data JSON,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for research tasks
CREATE INDEX IF NOT EXISTS idx_research_tasks_status 
  ON exa_research_tasks(status, created_at);
CREATE INDEX IF NOT EXISTS idx_research_tasks_project 
  ON exa_research_tasks(project_context, created_at);
CREATE INDEX IF NOT EXISTS idx_research_events_task 
  ON exa_research_events(task_id, created_at);

-- Virtual view for active research tasks
CREATE OR REPLACE VIEW v_active_research_tasks AS
SELECT 
  id,
  instructions,
  model,
  status,
  progress_message,
  EXTRACT(EPOCH FROM (CAST(CURRENT_TIMESTAMP AS TIMESTAMP) - created_at)) as elapsed_seconds,
  estimated_cost,
  retry_count,
  project_context
FROM exa_research_tasks
WHERE status IN ('pending', 'running', 'polling')
ORDER BY created_at DESC;
`

// dropSchema drops all knowledge-related tables
const dropSchema = `
DROP TABLE IF EXISTS docset_content;
DROP TABLE IF EXISTS docsets;
DROP SEQUENCE IF EXISTS docset_content_seq;
`
