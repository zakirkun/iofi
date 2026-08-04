-- Reference documents the Founder uploads: specifications, standards,
-- contracts, runbooks. Distinct from `memories`, which agents write about
-- themselves — these are authoritative and a disagreement between the two is
-- resolved in the document's favour.

CREATE TABLE knowledge_docs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope       text NOT NULL DEFAULT 'global',  -- global | role | project
    scope_ref   text NOT NULL DEFAULT '',        -- role id or board id; '' for global
    title       text NOT NULL,
    filename    text NOT NULL DEFAULT '',
    mime        text NOT NULL DEFAULT '',
    bytes       bigint NOT NULL DEFAULT 0,
    -- Content hash, so re-uploading the same file into the same scope replaces
    -- the document instead of adding a second copy that search returns twice.
    sha256      text NOT NULL DEFAULT '',
    source_path text NOT NULL DEFAULT '',        -- <DataDir>/knowledge/<id><ext>
    extractor   text NOT NULL DEFAULT '',        -- docling | plain
    -- Extraction runs in the background: a scanned PDF takes longer than any
    -- HTTP request should, so the upload returns before the text exists.
    state       text NOT NULL DEFAULT 'pending', -- pending | ready | failed
    error       text NOT NULL DEFAULT '',
    content     text NOT NULL DEFAULT '',        -- extracted markdown, whole document
    tags        text[] NOT NULL DEFAULT '{}',
    uploaded_by text NOT NULL DEFAULT 'founder',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX knowledge_docs_scope_idx ON knowledge_docs(scope, scope_ref);
CREATE UNIQUE INDEX knowledge_docs_sha_idx
    ON knowledge_docs(scope, scope_ref, sha256) WHERE sha256 <> '';

-- One row per retrievable passage. Whole documents are too large to put in a
-- prompt and too coarse to rank, so recall works on chunks and the document is
-- fetched only when a chunk is not enough.
CREATE TABLE knowledge_chunks (
    id        bigserial PRIMARY KEY,
    doc_id    uuid NOT NULL REFERENCES knowledge_docs(id) ON DELETE CASCADE,
    idx       int  NOT NULL,               -- position in the document, 0-based
    heading   text NOT NULL DEFAULT '',    -- nearest heading, so a hit can be cited
    content   text NOT NULL,
    embedding vector(1536),
    UNIQUE (doc_id, idx)
);
CREATE INDEX knowledge_chunks_doc_idx ON knowledge_chunks(doc_id, idx);
CREATE INDEX knowledge_chunks_vec_idx ON knowledge_chunks USING hnsw (embedding vector_cosine_ops);

-- Every role gains the three read tools. Uploaded reference material is the
-- Founder's own instruction to the company; withholding it from a role would
-- mean that role works against a specification it is not allowed to read.
UPDATE roles SET tool_policy = jsonb_set(
    tool_policy, '{allow}',
    (tool_policy->'allow') || '["knowledge_search","knowledge_get","knowledge_list"]'::jsonb)
WHERE tool_policy ? 'allow' AND jsonb_typeof(tool_policy->'allow') = 'array';
