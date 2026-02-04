-- name: CreateTeam :one
INSERT INTO teams (
    name,
    description,
    default_escalation_policy_id,
    default_channel,
    assigned_sites,
    assigned_pops,
    metadata
) VALUES (
    @name, @description, @default_escalation_policy_id, @default_channel, @assigned_sites, @assigned_pops, @metadata
)
RETURNING *;

-- name: GetTeam :one
SELECT * FROM teams
WHERE id = @id;

-- name: ListTeams :many
SELECT * FROM teams
WHERE
    (COALESCE(NULLIF(@name_filter, ''), '') = '' OR name ILIKE '%' || @name_filter || '%')
    AND (COALESCE(cardinality(@sites_filter::text[]), 0) = 0 OR assigned_sites && @sites_filter::text[])
    AND (COALESCE(cardinality(@pops_filter::text[]), 0) = 0 OR assigned_pops && @pops_filter::text[])
ORDER BY name ASC
LIMIT @lim OFFSET @off;

-- name: ListTeamsCount :one
SELECT COUNT(*) FROM teams
WHERE
    (COALESCE(NULLIF(@name_filter, ''), '') = '' OR name ILIKE '%' || @name_filter || '%')
    AND (COALESCE(cardinality(@sites_filter::text[]), 0) = 0 OR assigned_sites && @sites_filter::text[])
    AND (COALESCE(cardinality(@pops_filter::text[]), 0) = 0 OR assigned_pops && @pops_filter::text[]);

-- name: UpdateTeam :one
UPDATE teams
SET
    name = COALESCE(NULLIF(@name, ''), name),
    description = COALESCE(@description, description),
    default_escalation_policy_id = @default_escalation_policy_id,
    default_channel = COALESCE(@default_channel, default_channel),
    assigned_sites = COALESCE(@assigned_sites, assigned_sites),
    assigned_pops = COALESCE(@assigned_pops, assigned_pops),
    metadata = COALESCE(@metadata, metadata),
    updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: DeleteTeam :exec
DELETE FROM teams
WHERE id = @id;

-- name: AddTeamMember :exec
INSERT INTO team_members (team_id, user_id, role, preferences)
VALUES (@team_id, @user_id, @role, @preferences)
ON CONFLICT (team_id, user_id)
DO UPDATE SET role = EXCLUDED.role, preferences = EXCLUDED.preferences;

-- name: RemoveTeamMember :exec
DELETE FROM team_members
WHERE team_id = @team_id AND user_id = @user_id;

-- name: UpdateTeamMember :exec
UPDATE team_members
SET role = @role, preferences = @preferences
WHERE team_id = @team_id AND user_id = @user_id;

-- name: GetTeamMembers :many
SELECT * FROM team_members
WHERE team_id = @team_id
ORDER BY joined_at ASC;

-- name: GetUserTeams :many
SELECT t.* FROM teams t
INNER JOIN team_members tm ON t.id = tm.team_id
WHERE tm.user_id = @user_id
ORDER BY t.name ASC;

-- name: GetTeamMember :one
SELECT * FROM team_members
WHERE team_id = @team_id AND user_id = @user_id;

-- name: GetTeamsByEscalationPolicy :many
SELECT * FROM teams
WHERE default_escalation_policy_id = @policy_id
ORDER BY name ASC;
