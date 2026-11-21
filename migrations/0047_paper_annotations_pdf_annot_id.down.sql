DROP INDEX IF EXISTS paper_annotations_pdf_annot_id_key;
ALTER TABLE paper_annotations DROP COLUMN IF EXISTS pdf_annot_id;
