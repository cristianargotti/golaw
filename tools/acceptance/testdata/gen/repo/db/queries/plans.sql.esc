-- name: ReadPlan :one
SELECT account, name, archived, since FROM plans WHERE account = $1;

-- name: ArchivePlan :execrows
UPDATE plans SET archived = TRUE WHERE account = $1;
