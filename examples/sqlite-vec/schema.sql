-- sqlite-vec schema for collections and vector storage
-- Load the vector extension
.load /usr/local/lib/sqlite/vec0

-- Collections table: organize vectors into named groups
-- All collections use the same embedding model dimension
CREATE TABLE IF NOT EXISTS collections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Vectors table: store embeddings with metadata
CREATE TABLE IF NOT EXISTS vectors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collection_id INTEGER NOT NULL,
    vector_blob BLOB NOT NULL,
    metadata TEXT, -- JSON for flexible metadata storage
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_vectors_collection ON vectors(collection_id);
CREATE INDEX IF NOT EXISTS idx_collections_name ON collections(name);

-- Example queries:
--
-- Insert a collection:
-- INSERT INTO collections (name) VALUES ('code_embeddings');
--
-- Insert a vector (example with 3D vector):
-- INSERT INTO vectors (collection_id, vector_blob, metadata)
-- VALUES (1, vec_f32(json_array(0.1, 0.2, 0.3)), '{"file": "main.go", "line": 42}');
--
-- Search within a collection (top 10 similar vectors):
-- SELECT v.id, v.metadata, vec_distance_cosine(v.vector_blob, vec_f32(?)) as distance
-- FROM vectors v
-- WHERE v.collection_id = ?
-- ORDER BY distance
-- LIMIT 10;
--
-- Search across ALL collections:
-- SELECT c.name as collection, v.id, v.metadata, vec_distance_cosine(v.vector_blob, vec_f32(?)) as distance
-- FROM vectors v
-- JOIN collections c ON v.collection_id = c.id
-- ORDER BY distance
-- LIMIT 10;
--
-- Search across all collections EXCEPT specific ones:
-- SELECT c.name as collection, v.id, v.metadata, vec_distance_cosine(v.vector_blob, vec_f32(?)) as distance
-- FROM vectors v
-- JOIN collections c ON v.collection_id = c.id
-- WHERE c.name NOT IN ('collection1', 'collection2')
-- ORDER BY distance
-- LIMIT 10;
