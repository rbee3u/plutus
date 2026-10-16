-- name: InsertEarnPosition :exec
INSERT INTO earn_positions (portfolio_id, report_id, position_key, product_ref, product_name, product_type, granularity, amount_basis, status, subscribed_at, maturity_at, details_json)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(report_id), sqlc.arg(position_key), sqlc.narg(product_ref), sqlc.narg(product_name), sqlc.arg(product_type), sqlc.arg(granularity), sqlc.arg(amount_basis), sqlc.narg(status), sqlc.narg(subscribed_at), sqlc.narg(maturity_at), sqlc.arg(details_json));

-- name: ListEarnPositionsByReport :many
SELECT * FROM earn_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id = sqlc.arg(report_id)
ORDER BY position_key;

-- name: ListEarnPositionsByReports :many
SELECT * FROM earn_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id IN (sqlc.slice('report_ids'))
ORDER BY report_id, position_key;
