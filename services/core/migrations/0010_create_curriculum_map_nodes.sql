-- Curriculum map (T-0006): metadata-only infrastructure for topic maps, learning objectives,
-- practical skills, and assessment rules. Holds editorial structure and identity metadata only
-- — never syllabus text, objective wording, topic labels, assessment text, questions, mark
-- schemes, or any derivative content. A node's editorial label/summary field is a placeholder
-- for a future private, approved authoring workflow; this migration seeds none.
--
-- Every node MUST reference an approved, syllabus-matched content_sources row (the T-0001
-- rights gate). This is enforced at the application layer before every write (Core is the sole
-- authority for the source gate — see services/core/internal/curriculummap), not by a DB
-- constraint, because "approved" and "syllabus-matched" are point-in-time facts about another
-- table that a bare FK cannot express.
CREATE TABLE IF NOT EXISTS curriculum_map_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    syllabus_id UUID NOT NULL REFERENCES syllabuses (id),
    parent_node_id UUID REFERENCES curriculum_map_nodes (id),
    node_kind TEXT NOT NULL CHECK (node_kind IN ('topic', 'objective', 'practical_skill', 'assessment_rule')),
    -- node_code is a stable editorial reference/code, unique within its syllabus.
    node_code TEXT NOT NULL,
    -- label is the editorial label/summary field for the future private approved workflow.
    label TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'verified', 'retired')),
    content_source_id UUID NOT NULL REFERENCES content_sources (id),
    -- source_locator is optional reference metadata (e.g. a section/page label) pointing at the
    -- approved source — never the source content itself.
    source_locator TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- No duplicate stable node code within the same syllabus.
CREATE UNIQUE INDEX IF NOT EXISTS uq_curriculum_map_nodes_syllabus_code
    ON curriculum_map_nodes (syllabus_id, node_code);

CREATE INDEX IF NOT EXISTS idx_curriculum_map_nodes_syllabus ON curriculum_map_nodes (syllabus_id);
CREATE INDEX IF NOT EXISTS idx_curriculum_map_nodes_parent ON curriculum_map_nodes (parent_node_id);
CREATE INDEX IF NOT EXISTS idx_curriculum_map_nodes_status ON curriculum_map_nodes (status);
CREATE INDEX IF NOT EXISTS idx_curriculum_map_nodes_source ON curriculum_map_nodes (content_source_id);

-- Parent-same-syllabus enforcement and cycle prevention are done at the application layer
-- (services/core/internal/curriculummap/postgres_store.go), inside the same transaction as the
-- write, using row locks on the parent chain. This keeps error mapping (invalid_parent) under
-- Core's control rather than parsing trigger-raised text, while still validating before commit.
