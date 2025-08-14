package knowledge

import "database/sql"

// ReflectionSchema contains the schema for reflection and context management tables
// These tables integrate with the existing knowledge database to provide
// context persistence and learning capabilities
const ReflectionSchema = `
-- Create sequences for reflection tables
CREATE SEQUENCE IF NOT EXISTS reflection_operation_seq;
CREATE SEQUENCE IF NOT EXISTS reflection_pattern_seq;
CREATE SEQUENCE IF NOT EXISTS reflection_score_seq;
CREATE SEQUENCE IF NOT EXISTS reflection_event_seq;

-- Sessions table (links to project context and knowledge searches)
CREATE TABLE IF NOT EXISTS reflection_sessions (
	session_id VARCHAR PRIMARY KEY,
	project_path VARCHAR,
	project_name VARCHAR,
	start_time TIMESTAMP,
	last_checkpoint TIMESTAMP,
	status VARCHAR DEFAULT 'active',
	metadata JSON,
	-- Link to knowledge context
	last_search_id TEXT REFERENCES exa_search_cache(id),
	last_docset_id TEXT REFERENCES docsets(id)
);

-- Operations tracking table (links to knowledge operations)
CREATE TABLE IF NOT EXISTS reflection_operations (
	id INTEGER PRIMARY KEY DEFAULT nextval('reflection_operation_seq'),
	session_id VARCHAR REFERENCES reflection_sessions(session_id),
	tool_name VARCHAR,
	operation_type VARCHAR,
	arguments JSON,
	result JSON,
	success BOOLEAN,
	timestamp TIMESTAMP,
	-- Link to knowledge operations
	search_id TEXT REFERENCES exa_search_cache(id),
	docset_content_id INTEGER REFERENCES docset_content(id),
	embedding REAL[] -- Embedding of the operation for similarity search
);

-- Context checkpoints table
CREATE TABLE IF NOT EXISTS reflection_checkpoints (
	checkpoint_id VARCHAR PRIMARY KEY,
	session_id VARCHAR REFERENCES reflection_sessions(session_id),
	timestamp TIMESTAMP,
	reason VARCHAR,
	working_memory JSON,
	context_score JSON,
	operations_snapshot JSON,
	-- Link to knowledge state at checkpoint
	knowledge_summary JSON
);

-- Pattern learning table (learns from knowledge access patterns)
CREATE TABLE IF NOT EXISTS reflection_patterns (
	pattern_id INTEGER PRIMARY KEY DEFAULT nextval('reflection_pattern_seq'),
	pattern_sequence JSON,
	occurrence_count INTEGER DEFAULT 1,
	success_rate DOUBLE,
	pattern_type VARCHAR, -- 'success' or 'failure'
	last_seen TIMESTAMP,
	project_context VARCHAR,
	-- Knowledge access pattern
	knowledge_pattern JSON -- Which docs/searches led to success
);

-- Working memory persistence
CREATE TABLE IF NOT EXISTS reflection_working_memory (
	session_id VARCHAR PRIMARY KEY REFERENCES reflection_sessions(session_id),
	current_task VARCHAR,
	sub_tasks JSON,
	completed_steps JSON,
	pending_steps JSON,
	blockers JSON,
	discoveries JSON,
	assumptions JSON,
	decisions JSON,
	last_updated TIMESTAMP,
	-- Current knowledge context
	active_docsets JSON,
	recent_searches JSON
);

-- Context scores over time (includes knowledge quality metrics)
CREATE TABLE IF NOT EXISTS reflection_context_scores (
	id INTEGER PRIMARY KEY DEFAULT nextval('reflection_score_seq'),
	session_id VARCHAR REFERENCES reflection_sessions(session_id),
	timestamp TIMESTAMP,
	completeness DOUBLE,
	relevance DOUBLE,
	confidence DOUBLE,
	clarity DOUBLE,
	code_understanding DOUBLE,
	knowledge_coverage DOUBLE, -- How well knowledge base covers the task
	depth INTEGER
);

-- Reflection events (when reflection was triggered)
CREATE TABLE IF NOT EXISTS reflection_events (
	id INTEGER PRIMARY KEY DEFAULT nextval('reflection_event_seq'),
	session_id VARCHAR REFERENCES reflection_sessions(session_id),
	timestamp TIMESTAMP,
	trigger_reason VARCHAR,
	prompt TEXT,
	response TEXT,
	context_score JSON,
	operation_count INTEGER,
	-- Knowledge context at reflection time
	knowledge_accessed JSON
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_reflection_session_timestamp ON reflection_operations(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_reflection_session_checkpoint ON reflection_checkpoints(session_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_reflection_pattern_type ON reflection_patterns(pattern_type, occurrence_count DESC);
CREATE INDEX IF NOT EXISTS idx_reflection_session_scores ON reflection_context_scores(session_id, timestamp DESC);

-- View to analyze knowledge access patterns during reflection
CREATE OR REPLACE VIEW v_reflection_knowledge_patterns AS
SELECT 
	ro.session_id,
	ro.tool_name,
	ro.timestamp,
	esc.query as search_query,
	dc.name as docset_item,
	d.name as docset_name,
	ro.success,
	rs.current_task
FROM reflection_operations ro
LEFT JOIN exa_search_cache esc ON ro.search_id = esc.id
LEFT JOIN docset_content dc ON ro.docset_content_id = dc.id
LEFT JOIN docsets d ON dc.docset_id = d.id
LEFT JOIN reflection_working_memory rs ON ro.session_id = rs.session_id
WHERE ro.tool_name IN ('Search', 'ExaSearch', 'GetContent', 'ExecuteQuery')
ORDER BY ro.timestamp DESC;

-- View to analyze reflection effectiveness
CREATE OR REPLACE VIEW v_reflection_effectiveness AS
SELECT 
	rs.session_id,
	rs.project_name,
	COUNT(DISTINCT re.id) as reflection_count,
	AVG(rcs.confidence) as avg_confidence,
	AVG(rcs.completeness) as avg_completeness,
	AVG(rcs.knowledge_coverage) as avg_knowledge_coverage,
	COUNT(DISTINCT ro.search_id) as unique_searches,
	COUNT(DISTINCT ro.docset_content_id) as unique_docs_accessed,
	SUM(CASE WHEN ro.success THEN 1 ELSE 0 END)::FLOAT / COUNT(ro.id) as success_rate
FROM reflection_sessions rs
LEFT JOIN reflection_events re ON rs.session_id = re.session_id
LEFT JOIN reflection_context_scores rcs ON rs.session_id = rcs.session_id
LEFT JOIN reflection_operations ro ON rs.session_id = ro.session_id
GROUP BY rs.session_id, rs.project_name;
`

// AddReflectionSchema adds reflection tables to an existing knowledge database
func AddReflectionSchema(db *sql.DB) error {
	_, err := db.Exec(ReflectionSchema)
	return err
}
