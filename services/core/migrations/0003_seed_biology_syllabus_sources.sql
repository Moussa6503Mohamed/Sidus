-- Seed metadata only, for the official Biology 0610 / 5090 syllabus links documented in
-- docs/content-provenance-register.md. No PDFs/extracts/derivative content. Rights fields
-- (owner, source_hash, licence_reference, allowed_audience) are intentionally left null:
-- they are not yet documented in the provenance register and must not be guessed. These
-- rows stay 'pending' until a reviewer supplies and approves the missing rights fields.
INSERT INTO content_sources (title, source_url, syllabus_code, permitted_use, status)
VALUES
    (
        'Cambridge IGCSE Biology 0610 syllabus',
        'https://www.cambridgeinternational.org/Images/697203-2026-2028-syllabus.pdf',
        '0610',
        'Link, version metadata, human-reviewed topic/objective mapping only',
        'pending'
    ),
    (
        'Cambridge O Level Biology 5090 syllabus',
        'https://www.cambridgeinternational.org/Images/697330-2026-2028-syllabus.pdf',
        '5090',
        'Link, version metadata, human-reviewed topic/objective mapping only',
        'pending'
    )
ON CONFLICT (source_url) DO NOTHING;
