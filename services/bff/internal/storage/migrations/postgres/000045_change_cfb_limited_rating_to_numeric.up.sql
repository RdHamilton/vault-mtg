-- Change CFB limited_rating from TEXT to REAL (PostgreSQL)
-- In PostgreSQL we can use ALTER COLUMN TYPE with a USING clause

ALTER TABLE cfb_ratings
    ALTER COLUMN limited_rating TYPE REAL
    USING CASE limited_rating
        WHEN 'A+' THEN 5.0
        WHEN 'A'  THEN 4.5
        WHEN 'A-' THEN 4.0
        WHEN 'B+' THEN 3.5
        WHEN 'B'  THEN 3.0
        WHEN 'B-' THEN 2.5
        WHEN 'C+' THEN 2.0
        WHEN 'C'  THEN 1.5
        WHEN 'C-' THEN 1.0
        WHEN 'D'  THEN 0.5
        WHEN 'F'  THEN 0.0
        ELSE CAST(limited_rating AS REAL)
    END;
