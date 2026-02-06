package site

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresStore_GetByCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "code", "site_type", "tier", "region", "country", "city", "address", "timezone",
			"primary_team_id", "secondary_team_id", "default_escalation_policy_id", "parent_site_id",
			"labels", "business_hours", "created_at", "updated_at",
		}).AddRow(
			"site-123", "NYC Datacenter", "NYC-DC1", "datacenter", 1, "us-east-1", "USA", "New York", "123 Main St", "America/New_York",
			nil, nil, nil, nil,
			[]byte(`{"env":"production"}`), []byte(`{"start":"09:00","end":"17:00","days":[1,2,3,4,5]}`), now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE code = \$1`).
			WithArgs("NYC-DC1").
			WillReturnRows(rows)

		site, err := store.GetByCode(ctx, "NYC-DC1")
		require.NoError(t, err)
		assert.Equal(t, "site-123", site.ID)
		assert.Equal(t, "NYC Datacenter", site.Name)
		assert.Equal(t, "NYC-DC1", site.Code)
		assert.Equal(t, SiteTypeDatacenter, site.SiteType)
		assert.NotNil(t, site.Tier)
		assert.Equal(t, 1, *site.Tier)
		assert.Equal(t, "us-east-1", site.Region)
		assert.Equal(t, "production", site.Labels["env"])
		assert.NotNil(t, site.BusinessHours)
		assert.Equal(t, "09:00", site.BusinessHours.Start)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE code = \$1`).
			WithArgs("NONEXISTENT").
			WillReturnRows(sqlmock.NewRows(nil))

		_, err := store.GetByCode(ctx, "NONEXISTENT")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSiteNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStore_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "code", "site_type", "tier", "region", "country", "city", "address", "timezone",
			"primary_team_id", "secondary_team_id", "default_escalation_policy_id", "parent_site_id",
			"labels", "business_hours", "created_at", "updated_at",
		}).AddRow(
			"site-123", "NYC Datacenter", "NYC-DC1", "datacenter", 2, "us-east-1", "USA", "New York", "123 Main St", "America/New_York",
			"team-1", "team-2", "policy-1", nil,
			[]byte(`{}`), nil, now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE id = \$1`).
			WithArgs("site-123").
			WillReturnRows(rows)

		site, err := store.GetByID(ctx, "site-123")
		require.NoError(t, err)
		assert.Equal(t, "site-123", site.ID)
		assert.Equal(t, "NYC Datacenter", site.Name)
		assert.NotNil(t, site.PrimaryTeamID)
		assert.Equal(t, "team-1", *site.PrimaryTeamID)
		assert.NotNil(t, site.SecondaryTeamID)
		assert.Equal(t, "team-2", *site.SecondaryTeamID)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE id = \$1`).
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows(nil))

		_, err := store.GetByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSiteNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStore_List(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success - no filters", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "code", "site_type", "tier", "region", "country", "city", "address", "timezone",
			"primary_team_id", "secondary_team_id", "default_escalation_policy_id", "parent_site_id",
			"labels", "business_hours", "created_at", "updated_at",
		}).AddRow(
			"site-1", "NYC Datacenter", "NYC-DC1", "datacenter", 1, "us-east-1", "USA", "New York", "", "America/New_York",
			nil, nil, nil, nil,
			[]byte(`{}`), nil, now, now,
		).AddRow(
			"site-2", "LAX POP", "LAX-POP1", "pop", nil, "us-west-2", "USA", "Los Angeles", "", "America/Los_Angeles",
			nil, nil, nil, nil,
			[]byte(`{}`), nil, now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE 1=1 ORDER BY name ASC LIMIT \$1`).
			WithArgs(51). // pageSize + 1
			WillReturnRows(rows)

		sites, nextToken, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, sites, 2)
		assert.Empty(t, nextToken)
		assert.Equal(t, "NYC Datacenter", sites[0].Name)
		assert.Equal(t, "LAX POP", sites[1].Name)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - with type filter", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "code", "site_type", "tier", "region", "country", "city", "address", "timezone",
			"primary_team_id", "secondary_team_id", "default_escalation_policy_id", "parent_site_id",
			"labels", "business_hours", "created_at", "updated_at",
		}).AddRow(
			"site-1", "NYC Datacenter", "NYC-DC1", "datacenter", 1, "us-east-1", "USA", "New York", "", "America/New_York",
			nil, nil, nil, nil,
			[]byte(`{}`), nil, now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE 1=1 AND site_type = \$1 ORDER BY name ASC LIMIT \$2`).
			WithArgs("datacenter", 51).
			WillReturnRows(rows)

		sites, _, err := store.List(ctx, &ListSitesFilter{SiteType: SiteTypeDatacenter})
		require.NoError(t, err)
		assert.Len(t, sites, 1)
		assert.Equal(t, "NYC Datacenter", sites[0].Name)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - with region filter", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "code", "site_type", "tier", "region", "country", "city", "address", "timezone",
			"primary_team_id", "secondary_team_id", "default_escalation_policy_id", "parent_site_id",
			"labels", "business_hours", "created_at", "updated_at",
		}).AddRow(
			"site-1", "NYC Datacenter", "NYC-DC1", "datacenter", 1, "us-east-1", "USA", "New York", "", "America/New_York",
			nil, nil, nil, nil,
			[]byte(`{}`), nil, now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE 1=1 AND region = \$1 ORDER BY name ASC LIMIT \$2`).
			WithArgs("us-east-1", 51).
			WillReturnRows(rows)

		sites, _, err := store.List(ctx, &ListSitesFilter{Region: "us-east-1"})
		require.NoError(t, err)
		assert.Len(t, sites, 1)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("pagination", func(t *testing.T) {
		now := time.Now()
		// Return more than pageSize to trigger next page token
		rows := sqlmock.NewRows([]string{
			"id", "name", "code", "site_type", "tier", "region", "country", "city", "address", "timezone",
			"primary_team_id", "secondary_team_id", "default_escalation_policy_id", "parent_site_id",
			"labels", "business_hours", "created_at", "updated_at",
		})
		for i := 0; i < 11; i++ {
			rows.AddRow(
				"site-"+string(rune('0'+i)), "Site "+string(rune('0'+i)), "CODE-"+string(rune('0'+i)), "datacenter", 1, "us-east-1", "USA", "City", "", "UTC",
				nil, nil, nil, nil,
				[]byte(`{}`), nil, now, now,
			)
		}

		mock.ExpectQuery(`SELECT (.+) FROM sites WHERE 1=1 ORDER BY name ASC LIMIT \$1`).
			WithArgs(11). // pageSize + 1
			WillReturnRows(rows)

		sites, nextToken, err := store.List(ctx, &ListSitesFilter{PageSize: 10})
		require.NoError(t, err)
		assert.Len(t, sites, 10)
		assert.NotEmpty(t, nextToken)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStore_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(`INSERT INTO sites`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		site := &Site{
			Name:     "NYC Datacenter",
			Code:     "NYC-DC1",
			SiteType: SiteTypeDatacenter,
			Region:   "us-east-1",
			Timezone: "America/New_York",
			Labels:   map[string]string{"env": "production"},
		}

		created, err := store.Create(ctx, site)
		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, "NYC Datacenter", created.Name)
		assert.Equal(t, "NYC-DC1", created.Code)
		assert.False(t, created.CreatedAt.IsZero())
		assert.False(t, created.UpdatedAt.IsZero())

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid - nil site", func(t *testing.T) {
		_, err := store.Create(ctx, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSite)
	})

	t.Run("invalid - empty code", func(t *testing.T) {
		site := &Site{
			Name: "Test Site",
		}
		_, err := store.Create(ctx, site)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSite)
	})

	t.Run("invalid - empty name", func(t *testing.T) {
		site := &Site{
			Code: "TEST-1",
		}
		_, err := store.Create(ctx, site)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSite)
	})
}

func TestPostgresStore_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(`UPDATE sites SET`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		site := &Site{
			ID:       "site-123",
			Name:     "Updated Datacenter",
			Code:     "NYC-DC1",
			SiteType: SiteTypeDatacenter,
			Region:   "us-east-2",
			Timezone: "America/New_York",
		}

		updated, err := store.Update(ctx, site)
		require.NoError(t, err)
		assert.Equal(t, "site-123", updated.ID)
		assert.Equal(t, "Updated Datacenter", updated.Name)
		assert.False(t, updated.UpdatedAt.IsZero())

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid - nil site", func(t *testing.T) {
		_, err := store.Update(ctx, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSite)
	})

	t.Run("invalid - empty id", func(t *testing.T) {
		site := &Site{
			Name: "Test Site",
			Code: "TEST-1",
		}
		_, err := store.Update(ctx, site)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSite)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(`UPDATE sites SET`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		site := &Site{
			ID:       "nonexistent",
			Name:     "Test Site",
			Code:     "TEST-1",
			Timezone: "UTC",
		}

		_, err := store.Update(ctx, site)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSiteNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStore_Delete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(`DELETE FROM sites WHERE id = \$1`).
			WithArgs("site-123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := store.Delete(ctx, "site-123")
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(`DELETE FROM sites WHERE id = \$1`).
			WithArgs("nonexistent").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := store.Delete(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSiteNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStore_GetTeamByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "description", "default_escalation_policy_id", "default_notification_channel_id",
			"created_at", "updated_at",
		}).AddRow(
			"team-123", "NOC Team", "Network Operations Center", "policy-1", "channel-1", now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM teams WHERE id = \$1`).
			WithArgs("team-123").
			WillReturnRows(rows)

		team, err := store.GetTeamByID(ctx, "team-123")
		require.NoError(t, err)
		assert.Equal(t, "team-123", team.ID)
		assert.Equal(t, "NOC Team", team.Name)
		assert.Equal(t, "Network Operations Center", team.Description)
		assert.NotNil(t, team.DefaultEscalationPolicyID)
		assert.Equal(t, "policy-1", *team.DefaultEscalationPolicyID)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM teams WHERE id = \$1`).
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows(nil))

		_, err := store.GetTeamByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTeamNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
