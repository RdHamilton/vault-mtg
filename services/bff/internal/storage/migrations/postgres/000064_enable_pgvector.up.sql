-- Enable pgvector extension for vector similarity search.
-- Required for future RAG/AI features that store and query embedding vectors.
-- Safe to run on a fresh DB or one that already has the extension installed.
CREATE EXTENSION IF NOT EXISTS vector;
