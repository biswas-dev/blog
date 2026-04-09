-- Paper research areas (separate from post categories and book genres)
CREATE TABLE IF NOT EXISTS paper_research_areas (
    area_id    SERIAL PRIMARY KEY,
    area_name  VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Papers: academic paper reviews
CREATE TABLE IF NOT EXISTS papers (
    paper_id         SERIAL PRIMARY KEY,
    user_id          INT REFERENCES Users(user_id),
    title            VARCHAR(500) NOT NULL,
    slug             VARCHAR(255) NOT NULL UNIQUE,
    paper_authors    VARCHAR(1000) NOT NULL DEFAULT '',
    abstract         TEXT DEFAULT '',
    paper_year       INT DEFAULT 0,
    conference       VARCHAR(255) DEFAULT '',
    doi              VARCHAR(255) DEFAULT '',
    arxiv_id         VARCHAR(50) DEFAULT '',
    pdf_file_url     TEXT DEFAULT '',
    pdf_file_size    BIGINT DEFAULT 0,
    cover_image_url  TEXT DEFAULT '',
    content          TEXT NOT NULL DEFAULT '',
    description      TEXT DEFAULT '',
    my_notes         TEXT DEFAULT '',
    rating           NUMERIC(2,1) DEFAULT 0 CHECK (rating >= 0 AND rating <= 5),
    is_published     BOOLEAN DEFAULT FALSE,
    publication_date TIMESTAMP,
    last_edit_date   TIMESTAMP,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Many-to-many research areas
CREATE TABLE IF NOT EXISTS paper_research_area_map (
    paper_id INT REFERENCES papers(paper_id) ON DELETE CASCADE,
    area_id  INT REFERENCES paper_research_areas(area_id) ON DELETE CASCADE,
    PRIMARY KEY (paper_id, area_id)
);

-- PDF annotations (page-based, with bounding box for visual highlights)
CREATE TABLE IF NOT EXISTS paper_annotations (
    annotation_id SERIAL PRIMARY KEY,
    paper_id      INT NOT NULL REFERENCES papers(paper_id) ON DELETE CASCADE,
    author_id     INT NOT NULL REFERENCES Users(user_id),
    page_number   INT NOT NULL,
    selected_text TEXT DEFAULT '',
    bounding_box  JSONB DEFAULT '{}',
    color         VARCHAR(20) DEFAULT 'yellow',
    note          TEXT DEFAULT '',
    is_public     BOOLEAN DEFAULT FALSE,
    sort_order    INT DEFAULT 0,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Threaded comments on annotations
CREATE TABLE IF NOT EXISTS paper_annotation_comments (
    comment_id        SERIAL PRIMARY KEY,
    annotation_id     INT NOT NULL REFERENCES paper_annotations(annotation_id) ON DELETE CASCADE,
    author_id         INT NOT NULL REFERENCES Users(user_id),
    parent_comment_id INT REFERENCES paper_annotation_comments(comment_id) ON DELETE CASCADE,
    content           TEXT NOT NULL,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Paper version tracking
CREATE TABLE IF NOT EXISTS paper_versions (
    id             SERIAL PRIMARY KEY,
    paper_id       INT NOT NULL REFERENCES papers(paper_id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    title          VARCHAR(500) NOT NULL,
    content        TEXT NOT NULL DEFAULT '',
    content_hash   VARCHAR(64) NOT NULL,
    created_by     INT NOT NULL REFERENCES Users(user_id),
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (paper_id, version_number)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_papers_user_id ON papers(user_id);
CREATE INDEX IF NOT EXISTS idx_papers_slug ON papers(slug);
CREATE INDEX IF NOT EXISTS idx_papers_published ON papers(is_published);
CREATE INDEX IF NOT EXISTS idx_papers_year ON papers(paper_year);
CREATE INDEX IF NOT EXISTS idx_papers_conference ON papers(conference);
CREATE INDEX IF NOT EXISTS idx_paper_area_map_paper ON paper_research_area_map(paper_id);
CREATE INDEX IF NOT EXISTS idx_paper_area_map_area ON paper_research_area_map(area_id);
CREATE INDEX IF NOT EXISTS idx_paper_annotations_paper ON paper_annotations(paper_id);
CREATE INDEX IF NOT EXISTS idx_paper_annotation_comments_ann ON paper_annotation_comments(annotation_id);
CREATE INDEX IF NOT EXISTS idx_paper_versions_paper ON paper_versions(paper_id);

-- Seed research areas
INSERT INTO paper_research_areas (area_name) VALUES
    ('Machine Learning'),
    ('Natural Language Processing'),
    ('Computer Vision'),
    ('Systems'),
    ('Distributed Systems'),
    ('Databases'),
    ('Security'),
    ('Programming Languages'),
    ('HCI'),
    ('Networking'),
    ('Theory'),
    ('AI Safety'),
    ('AI Agents'),
    ('Robotics'),
    ('Quantum Computing')
ON CONFLICT (area_name) DO NOTHING;
