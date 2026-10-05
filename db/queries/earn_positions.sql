-- name: InsertEarnPosition :exec
INSERT INTO earn_positions (report_id, portfolio_id, platform_position_ref, observation_json)
VALUES (sqlc.arg(report_id), sqlc.arg(portfolio_id), sqlc.arg(platform_position_ref), sqlc.arg(observation_json));

-- name: ListEarnPositionsByReport :many
SELECT * FROM earn_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id = sqlc.arg(report_id)
ORDER BY platform_position_ref;

-- name: GetEarnDetailsCollected :one
SELECT earn_details_collected FROM account_reports
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);
