-- Existing inferred memories no longer require manual confirmation.
UPDATE user_memories
SET status = 'active',
    updated_at = CURRENT_TIMESTAMP(3)
WHERE source_type = 'inferred'
  AND status = 'candidate';
