package equipment

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresStore_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(`INSERT INTO equipment_types`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		eq := &EquipmentType{
			Name:        "router",
			Category:    CategoryNetwork,
			Vendor:      "cisco",
			Criticality: 5,
			Metadata:    map[string]string{"type": "core"},
		}

		created, err := store.Create(ctx, eq)
		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, "router", created.Name)
		assert.Equal(t, CategoryNetwork, created.Category)
		assert.Equal(t, 5, created.Criticality)
		assert.False(t, created.CreatedAt.IsZero())
		assert.False(t, created.UpdatedAt.IsZero())

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - default criticality", func(t *testing.T) {
		mock.ExpectExec(`INSERT INTO equipment_types`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		eq := &EquipmentType{
			Name:        "switch",
			Category:    CategoryNetwork,
			Criticality: 0, // Invalid, should default to 3
		}

		created, err := store.Create(ctx, eq)
		require.NoError(t, err)
		assert.Equal(t, 3, created.Criticality)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid - nil equipment type", func(t *testing.T) {
		_, err := store.Create(ctx, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidEquipmentType)
	})

	t.Run("invalid - empty name", func(t *testing.T) {
		eq := &EquipmentType{
			Category: CategoryNetwork,
		}
		_, err := store.Create(ctx, eq)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidEquipmentType)
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
			"id", "name", "category", "vendor", "criticality", "default_team_id",
			"escalation_policy", "routing_rules", "metadata", "created_at", "updated_at",
		}).AddRow(
			"eq-123", "router", "network", "cisco", 5, "team-1",
			"policy-1", []byte(`["rule-1","rule-2"]`), []byte(`{"type":"core"}`), now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE id = \$1`).
			WithArgs("eq-123").
			WillReturnRows(rows)

		eq, err := store.GetByID(ctx, "eq-123")
		require.NoError(t, err)
		assert.Equal(t, "eq-123", eq.ID)
		assert.Equal(t, "router", eq.Name)
		assert.Equal(t, Category("network"), eq.Category)
		assert.Equal(t, "cisco", eq.Vendor)
		assert.Equal(t, 5, eq.Criticality)
		assert.Equal(t, "team-1", eq.DefaultTeamID)
		assert.Equal(t, "policy-1", eq.EscalationPolicy)
		assert.Len(t, eq.RoutingRules, 2)
		assert.Equal(t, "core", eq.Metadata["type"])

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - with null fields", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "category", "vendor", "criticality", "default_team_id",
			"escalation_policy", "routing_rules", "metadata", "created_at", "updated_at",
		}).AddRow(
			"eq-456", "switch", "network", nil, 3, nil,
			nil, nil, nil, now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE id = \$1`).
			WithArgs("eq-456").
			WillReturnRows(rows)

		eq, err := store.GetByID(ctx, "eq-456")
		require.NoError(t, err)
		assert.Equal(t, "eq-456", eq.ID)
		assert.Equal(t, "switch", eq.Name)
		assert.Empty(t, eq.Vendor)
		assert.Empty(t, eq.DefaultTeamID)
		assert.NotNil(t, eq.Metadata)
		assert.Len(t, eq.RoutingRules, 0)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE id = \$1`).
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows(nil))

		_, err := store.GetByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEquipmentTypeNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStore_GetByName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "category", "vendor", "criticality", "default_team_id",
			"escalation_policy", "routing_rules", "metadata", "created_at", "updated_at",
		}).AddRow(
			"eq-123", "router", "network", "cisco", 5, nil,
			nil, []byte(`[]`), []byte(`{}`), now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE LOWER\(name\) = LOWER\(\$1\)`).
			WithArgs("Router"). // Case insensitive lookup
			WillReturnRows(rows)

		eq, err := store.GetByName(ctx, "Router")
		require.NoError(t, err)
		assert.Equal(t, "eq-123", eq.ID)
		assert.Equal(t, "router", eq.Name)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE LOWER\(name\) = LOWER\(\$1\)`).
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows(nil))

		_, err := store.GetByName(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEquipmentTypeNotFound)

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
			"id", "name", "category", "vendor", "criticality", "default_team_id",
			"escalation_policy", "routing_rules", "metadata", "created_at", "updated_at",
		}).AddRow(
			"eq-1", "router", "network", "cisco", 5, nil,
			nil, []byte(`[]`), []byte(`{}`), now, now,
		).AddRow(
			"eq-2", "switch", "network", "juniper", 4, nil,
			nil, []byte(`[]`), []byte(`{}`), now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE 1=1 ORDER BY criticality DESC, name ASC LIMIT \$1`).
			WithArgs(51). // pageSize + 1
			WillReturnRows(rows)

		types, nextToken, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, types, 2)
		assert.Empty(t, nextToken)
		assert.Equal(t, "router", types[0].Name)
		assert.Equal(t, "switch", types[1].Name)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - with category filter", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "category", "vendor", "criticality", "default_team_id",
			"escalation_policy", "routing_rules", "metadata", "created_at", "updated_at",
		}).AddRow(
			"eq-1", "router", "network", "cisco", 5, nil,
			nil, []byte(`[]`), []byte(`{}`), now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE 1=1 AND category = \$1 ORDER BY criticality DESC, name ASC LIMIT \$2`).
			WithArgs("network", 51).
			WillReturnRows(rows)

		types, _, err := store.List(ctx, &ListEquipmentTypesFilter{Category: CategoryNetwork})
		require.NoError(t, err)
		assert.Len(t, types, 1)
		assert.Equal(t, "router", types[0].Name)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - with vendor filter", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "category", "vendor", "criticality", "default_team_id",
			"escalation_policy", "routing_rules", "metadata", "created_at", "updated_at",
		}).AddRow(
			"eq-1", "router", "network", "cisco", 5, nil,
			nil, []byte(`[]`), []byte(`{}`), now, now,
		)

		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE 1=1 AND vendor = \$1 ORDER BY criticality DESC, name ASC LIMIT \$2`).
			WithArgs("cisco", 51).
			WillReturnRows(rows)

		types, _, err := store.List(ctx, &ListEquipmentTypesFilter{Vendor: "cisco"})
		require.NoError(t, err)
		assert.Len(t, types, 1)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("pagination", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "name", "category", "vendor", "criticality", "default_team_id",
			"escalation_policy", "routing_rules", "metadata", "created_at", "updated_at",
		})
		for i := 0; i < 11; i++ {
			rows.AddRow(
				"eq-"+string(rune('0'+i)), "device-"+string(rune('0'+i)), "network", nil, 3, nil,
				nil, []byte(`[]`), []byte(`{}`), now, now,
			)
		}

		mock.ExpectQuery(`SELECT (.+) FROM equipment_types WHERE 1=1 ORDER BY criticality DESC, name ASC LIMIT \$1`).
			WithArgs(11). // pageSize + 1
			WillReturnRows(rows)

		types, nextToken, err := store.List(ctx, &ListEquipmentTypesFilter{PageSize: 10})
		require.NoError(t, err)
		assert.Len(t, types, 10)
		assert.NotEmpty(t, nextToken)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStore_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewPostgresStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(`UPDATE equipment_types SET`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		eq := &EquipmentType{
			ID:          "eq-123",
			Name:        "updated_router",
			Category:    CategoryNetwork,
			Vendor:      "juniper",
			Criticality: 4,
		}

		updated, err := store.Update(ctx, eq)
		require.NoError(t, err)
		assert.Equal(t, "eq-123", updated.ID)
		assert.Equal(t, "updated_router", updated.Name)
		assert.False(t, updated.UpdatedAt.IsZero())

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid - nil equipment type", func(t *testing.T) {
		_, err := store.Update(ctx, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidEquipmentType)
	})

	t.Run("invalid - empty id", func(t *testing.T) {
		eq := &EquipmentType{
			Name:     "router",
			Category: CategoryNetwork,
		}
		_, err := store.Update(ctx, eq)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidEquipmentType)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(`UPDATE equipment_types SET`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		eq := &EquipmentType{
			ID:          "nonexistent",
			Name:        "router",
			Category:    CategoryNetwork,
			Criticality: 3,
		}

		_, err := store.Update(ctx, eq)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEquipmentTypeNotFound)

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
		mock.ExpectExec(`DELETE FROM equipment_types WHERE id = \$1`).
			WithArgs("eq-123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := store.Delete(ctx, "eq-123")
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(`DELETE FROM equipment_types WHERE id = \$1`).
			WithArgs("nonexistent").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := store.Delete(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEquipmentTypeNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
