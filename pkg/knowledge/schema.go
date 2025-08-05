package knowledge

const createSchema = `
-- Create sequences
CREATE SEQUENCE IF NOT EXISTS docset_content_seq;

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

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_docset_content_name ON docset_content(name);
CREATE INDEX IF NOT EXISTS idx_docset_content_type ON docset_content(docset_id, type);
`

// dropSchema drops all knowledge-related tables
const dropSchema = `
DROP TABLE IF EXISTS docset_content;
DROP TABLE IF EXISTS docsets;
DROP SEQUENCE IF EXISTS docset_content_seq;
`
