package carrier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStore_Create(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	t.Run("create valid carrier", func(t *testing.T) {
		carrier := &Carrier{
			Name:     "Level3",
			ASN:      3356,
			Type:     CarrierTypeTransit,
			Priority: 1,
			Contacts: []Contact{
				{Name: "NOC", Email: "noc@level3.com", Primary: true},
			},
			NOCEmail: "noc@level3.com",
		}

		created, err := store.Create(ctx, carrier)
		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, "Level3", created.Name)
		assert.Equal(t, 3356, created.ASN)
		assert.NotZero(t, created.CreatedAt)
		assert.NotZero(t, created.UpdatedAt)
	})

	t.Run("create carrier with nil returns error", func(t *testing.T) {
		_, err := store.Create(ctx, nil)
		assert.ErrorIs(t, err, ErrInvalidCarrier)
	})

	t.Run("create carrier without name returns error", func(t *testing.T) {
		carrier := &Carrier{ASN: 12345}
		_, err := store.Create(ctx, carrier)
		assert.ErrorIs(t, err, ErrInvalidCarrier)
	})

	t.Run("create carrier without ASN returns error", func(t *testing.T) {
		carrier := &Carrier{Name: "TestCarrier"}
		_, err := store.Create(ctx, carrier)
		assert.ErrorIs(t, err, ErrInvalidCarrier)
	})

	t.Run("create carrier with duplicate ASN returns error", func(t *testing.T) {
		carrier := &Carrier{
			Name: "Cogent",
			ASN:  3356, // Same as Level3
			Type: CarrierTypeTransit,
		}
		_, err := store.Create(ctx, carrier)
		assert.ErrorIs(t, err, ErrDuplicateASN)
	})

	t.Run("create carrier with duplicate name returns error", func(t *testing.T) {
		carrier := &Carrier{
			Name: "Level3", // Same name
			ASN:  174,      // Different ASN
			Type: CarrierTypeTransit,
		}
		_, err := store.Create(ctx, carrier)
		assert.ErrorIs(t, err, ErrDuplicateName)
	})
}

func TestInMemoryStore_GetByID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	carrier := &Carrier{
		Name:     "Cogent",
		ASN:      174,
		Type:     CarrierTypeTransit,
		Priority: 2,
	}
	created, err := store.Create(ctx, carrier)
	require.NoError(t, err)

	t.Run("get existing carrier by ID", func(t *testing.T) {
		found, err := store.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "Cogent", found.Name)
		assert.Equal(t, 174, found.ASN)
	})

	t.Run("get non-existent carrier returns not found", func(t *testing.T) {
		_, err := store.GetByID(ctx, "non-existent-id")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestInMemoryStore_GetByASN(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	carrier := &Carrier{
		Name:     "Hurricane Electric",
		ASN:      6939,
		Type:     CarrierTypePeering,
		Priority: 3,
	}
	_, err := store.Create(ctx, carrier)
	require.NoError(t, err)

	t.Run("get existing carrier by ASN", func(t *testing.T) {
		found, err := store.GetByASN(ctx, 6939)
		require.NoError(t, err)
		assert.Equal(t, "Hurricane Electric", found.Name)
		assert.Equal(t, 6939, found.ASN)
	})

	t.Run("get non-existent ASN returns not found", func(t *testing.T) {
		_, err := store.GetByASN(ctx, 99999)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestInMemoryStore_GetByName(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	carrier := &Carrier{
		Name:     "NTT",
		ASN:      2914,
		Type:     CarrierTypeTransit,
		Priority: 2,
	}
	_, err := store.Create(ctx, carrier)
	require.NoError(t, err)

	t.Run("get existing carrier by name", func(t *testing.T) {
		found, err := store.GetByName(ctx, "NTT")
		require.NoError(t, err)
		assert.Equal(t, 2914, found.ASN)
	})

	t.Run("get carrier by name case-insensitive", func(t *testing.T) {
		found, err := store.GetByName(ctx, "ntt")
		require.NoError(t, err)
		assert.Equal(t, 2914, found.ASN)
	})

	t.Run("get non-existent name returns not found", func(t *testing.T) {
		_, err := store.GetByName(ctx, "NonExistent")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestInMemoryStore_List(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create multiple carriers
	carriers := []*Carrier{
		{Name: "Carrier1", ASN: 1001, Type: CarrierTypeTransit, Priority: 1},
		{Name: "Carrier2", ASN: 1002, Type: CarrierTypePeering, Priority: 3},
		{Name: "Carrier3", ASN: 1003, Type: CarrierTypeTransit, Priority: 2},
		{Name: "TestPeer", ASN: 1004, Type: CarrierTypePeering, Priority: 5},
	}
	for _, c := range carriers {
		_, err := store.Create(ctx, c)
		require.NoError(t, err)
	}

	t.Run("list all carriers", func(t *testing.T) {
		list, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, list, 4)
	})

	t.Run("list carriers sorted by priority", func(t *testing.T) {
		list, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, list[0].Priority)
		assert.Equal(t, 2, list[1].Priority)
	})

	t.Run("filter by type", func(t *testing.T) {
		list, err := store.List(ctx, &CarrierFilter{Type: CarrierTypeTransit})
		require.NoError(t, err)
		assert.Len(t, list, 2)
		for _, c := range list {
			assert.Equal(t, CarrierTypeTransit, c.Type)
		}
	})

	t.Run("filter by name contains", func(t *testing.T) {
		list, err := store.List(ctx, &CarrierFilter{NameContains: "Test"})
		require.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, "TestPeer", list[0].Name)
	})

	t.Run("pagination limits results", func(t *testing.T) {
		list, err := store.List(ctx, &CarrierFilter{PageSize: 2})
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
}

func TestInMemoryStore_Update(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	carrier := &Carrier{
		Name:     "UpdateTest",
		ASN:      2001,
		Type:     CarrierTypePeering,
		Priority: 5,
	}
	created, err := store.Create(ctx, carrier)
	require.NoError(t, err)

	t.Run("update existing carrier", func(t *testing.T) {
		created.Priority = 2
		created.NOCEmail = "noc@updated.com"

		updated, err := store.Update(ctx, created)
		require.NoError(t, err)
		assert.Equal(t, 2, updated.Priority)
		assert.Equal(t, "noc@updated.com", updated.NOCEmail)
	})

	t.Run("update non-existent carrier returns not found", func(t *testing.T) {
		nonExistent := &Carrier{ID: "non-existent", Name: "Test", ASN: 9999}
		_, err := store.Update(ctx, nonExistent)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("update with nil returns error", func(t *testing.T) {
		_, err := store.Update(ctx, nil)
		assert.ErrorIs(t, err, ErrInvalidCarrier)
	})
}

func TestInMemoryStore_Delete(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	carrier := &Carrier{
		Name: "DeleteTest",
		ASN:  3001,
		Type: CarrierTypePeering,
	}
	created, err := store.Create(ctx, carrier)
	require.NoError(t, err)

	t.Run("delete existing carrier", func(t *testing.T) {
		err := store.Delete(ctx, created.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = store.GetByID(ctx, created.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("delete non-existent carrier returns not found", func(t *testing.T) {
		err := store.Delete(ctx, "non-existent-id")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestCarrier_ToProto(t *testing.T) {
	carrier := &Carrier{
		ID:               "test-id",
		Name:             "TestCarrier",
		ASN:              12345,
		NOCEmail:         "noc@test.com",
		NOCPhone:         "+1-555-1234",
		NOCPortalURL:     "https://noc.test.com",
		TeamID:           "team-123",
		AutoTicket:       true,
		TicketProviderID: "jira",
	}

	pb := carrier.ToProto()
	assert.Equal(t, "test-id", pb.Id)
	assert.Equal(t, "TestCarrier", pb.Name)
	assert.Equal(t, "12345", pb.Asn)
	assert.Equal(t, "noc@test.com", pb.NocEmail)
	assert.Equal(t, "+1-555-1234", pb.NocPhone)
	assert.Equal(t, "https://noc.test.com", pb.NocPortalUrl)
	assert.Equal(t, "team-123", pb.TeamId)
	assert.True(t, pb.AutoTicket)
	assert.Equal(t, "jira", pb.TicketProviderId)
}

func TestFromProto(t *testing.T) {
	t.Run("nil proto returns nil", func(t *testing.T) {
		result := FromProto(nil)
		assert.Nil(t, result)
	})
}
