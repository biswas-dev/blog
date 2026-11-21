-- Highlights saved inside the PDF by PdfCraft are mirrored into paper_annotations, keyed by
-- the annotation's /NM. NULL = a legacy (pre-PdfCraft) highlight, which the sync never touches.
ALTER TABLE paper_annotations ADD COLUMN IF NOT EXISTS pdf_annot_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS paper_annotations_pdf_annot_id_key
    ON paper_annotations (paper_id, pdf_annot_id) WHERE pdf_annot_id IS NOT NULL;
