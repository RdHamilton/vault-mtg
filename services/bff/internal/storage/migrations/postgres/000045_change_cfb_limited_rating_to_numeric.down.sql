-- Revert CFB limited_rating from REAL back to TEXT (PostgreSQL)
ALTER TABLE cfb_ratings
    ALTER COLUMN limited_rating TYPE TEXT
    USING CASE
        WHEN limited_rating IS NULL   THEN NULL
        WHEN limited_rating >= 4.75   THEN 'A+'
        WHEN limited_rating >= 4.25   THEN 'A'
        WHEN limited_rating >= 3.75   THEN 'A-'
        WHEN limited_rating >= 3.25   THEN 'B+'
        WHEN limited_rating >= 2.75   THEN 'B'
        WHEN limited_rating >= 2.25   THEN 'B-'
        WHEN limited_rating >= 1.75   THEN 'C+'
        WHEN limited_rating >= 1.25   THEN 'C'
        WHEN limited_rating >= 0.75   THEN 'C-'
        WHEN limited_rating >= 0.25   THEN 'D'
        ELSE 'F'
    END;
