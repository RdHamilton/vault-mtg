-- Remove pgvector extension.
-- WARNING: this will fail if any columns of type vector exist in the database.
-- Ensure all vector columns are dropped before running this rollback.
DROP EXTENSION IF EXISTS vector;
