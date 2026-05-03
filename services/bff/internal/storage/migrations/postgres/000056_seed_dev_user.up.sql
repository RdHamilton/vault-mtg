-- Seed a default development user so the API keys endpoint works on fresh installs.
-- This user is only inserted if no users exist yet; it is safe to run in production
-- (the WHERE NOT EXISTS guard ensures it is a no-op on live databases).
INSERT INTO users (email, subscription_status)
SELECT 'dev@mtga-companion.local', 'pro'
WHERE NOT EXISTS (SELECT 1 FROM users LIMIT 1);
