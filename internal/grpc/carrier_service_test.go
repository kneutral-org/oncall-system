package grpc

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kneutral-org/alerting-system/internal/carrier"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func setupCarrierService(t *testing.T) *CarrierService {
	store := carrier.NewInMemoryStore()
	logger := zerolog.Nop()
	return NewCarrierService(store, logger)
}

func TestCarrierService_CreateCarrier(t *testing.T) {
	svc := setupCarrierService(t)
	ctx := context.Background()

	t.Run("create valid carrier", func(t *testing.T) {
		req := &routingv1.CreateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Name:     "Level3",
				Asn:      "3356",
				NocEmail: "noc@level3.com",
			},
		}

		resp, err := svc.CreateCarrier(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Id)
		assert.Equal(t, "Level3", resp.Name)
		assert.Equal(t, "3356", resp.Asn)
		assert.Equal(t, "noc@level3.com", resp.NocEmail)
	})

	t.Run("create carrier with nil request", func(t *testing.T) {
		req := &routingv1.CreateCarrierRequest{}
		_, err := svc.CreateCarrier(ctx, req)
		assert.Error(t, err)
	})

	t.Run("create carrier without name", func(t *testing.T) {
		req := &routingv1.CreateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Asn: "12345",
			},
		}
		_, err := svc.CreateCarrier(ctx, req)
		assert.Error(t, err)
	})

	t.Run("create carrier without ASN", func(t *testing.T) {
		req := &routingv1.CreateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Name: "TestCarrier",
			},
		}
		_, err := svc.CreateCarrier(ctx, req)
		assert.Error(t, err)
	})

	t.Run("create carrier with duplicate ASN", func(t *testing.T) {
		// First carrier already created with ASN 3356
		req := &routingv1.CreateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Name: "DuplicateASN",
				Asn:  "3356",
			},
		}
		_, err := svc.CreateCarrier(ctx, req)
		assert.Error(t, err)
	})
}

func TestCarrierService_GetCarrier(t *testing.T) {
	svc := setupCarrierService(t)
	ctx := context.Background()

	// Create a carrier first
	createReq := &routingv1.CreateCarrierRequest{
		Carrier: &routingv1.CarrierConfig{
			Name:     "Cogent",
			Asn:      "174",
			NocEmail: "noc@cogent.com",
		},
	}
	created, err := svc.CreateCarrier(ctx, createReq)
	require.NoError(t, err)

	t.Run("get existing carrier", func(t *testing.T) {
		req := &routingv1.GetCarrierRequest{Id: created.Id}
		resp, err := svc.GetCarrier(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, created.Id, resp.Id)
		assert.Equal(t, "Cogent", resp.Name)
		assert.Equal(t, "174", resp.Asn)
	})

	t.Run("get carrier without ID", func(t *testing.T) {
		req := &routingv1.GetCarrierRequest{}
		_, err := svc.GetCarrier(ctx, req)
		assert.Error(t, err)
	})

	t.Run("get non-existent carrier", func(t *testing.T) {
		req := &routingv1.GetCarrierRequest{Id: "non-existent"}
		_, err := svc.GetCarrier(ctx, req)
		assert.Error(t, err)
	})
}

func TestCarrierService_GetCarrierByASN(t *testing.T) {
	svc := setupCarrierService(t)
	ctx := context.Background()

	// Create a carrier first
	createReq := &routingv1.CreateCarrierRequest{
		Carrier: &routingv1.CarrierConfig{
			Name: "Hurricane Electric",
			Asn:  "6939",
		},
	}
	_, err := svc.CreateCarrier(ctx, createReq)
	require.NoError(t, err)

	t.Run("get carrier by ASN", func(t *testing.T) {
		req := &routingv1.GetCarrierByASNRequest{Asn: "6939"}
		resp, err := svc.GetCarrierByASN(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "Hurricane Electric", resp.Name)
		assert.Equal(t, "6939", resp.Asn)
	})

	t.Run("get carrier by ASN without ASN", func(t *testing.T) {
		req := &routingv1.GetCarrierByASNRequest{}
		_, err := svc.GetCarrierByASN(ctx, req)
		assert.Error(t, err)
	})

	t.Run("get carrier by invalid ASN", func(t *testing.T) {
		req := &routingv1.GetCarrierByASNRequest{Asn: "invalid"}
		_, err := svc.GetCarrierByASN(ctx, req)
		assert.Error(t, err)
	})

	t.Run("get carrier by non-existent ASN", func(t *testing.T) {
		req := &routingv1.GetCarrierByASNRequest{Asn: "99999"}
		_, err := svc.GetCarrierByASN(ctx, req)
		assert.Error(t, err)
	})
}

func TestCarrierService_ListCarriers(t *testing.T) {
	svc := setupCarrierService(t)
	ctx := context.Background()

	// Create multiple carriers
	carriers := []struct {
		name string
		asn  string
	}{
		{"Carrier1", "1001"},
		{"Carrier2", "1002"},
		{"Carrier3", "1003"},
	}
	for _, c := range carriers {
		req := &routingv1.CreateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Name: c.name,
				Asn:  c.asn,
			},
		}
		_, err := svc.CreateCarrier(ctx, req)
		require.NoError(t, err)
	}

	t.Run("list all carriers", func(t *testing.T) {
		req := &routingv1.ListCarriersRequest{}
		resp, err := svc.ListCarriers(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.Carriers, 3)
	})

	t.Run("list with page size", func(t *testing.T) {
		req := &routingv1.ListCarriersRequest{PageSize: 2}
		resp, err := svc.ListCarriers(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.Carriers, 2)
	})
}

func TestCarrierService_UpdateCarrier(t *testing.T) {
	svc := setupCarrierService(t)
	ctx := context.Background()

	// Create a carrier first
	createReq := &routingv1.CreateCarrierRequest{
		Carrier: &routingv1.CarrierConfig{
			Name:     "UpdateTest",
			Asn:      "2001",
			NocEmail: "old@email.com",
		},
	}
	created, err := svc.CreateCarrier(ctx, createReq)
	require.NoError(t, err)

	t.Run("update carrier", func(t *testing.T) {
		req := &routingv1.UpdateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Id:       created.Id,
				Name:     "UpdateTest",
				Asn:      "2001",
				NocEmail: "new@email.com",
			},
		}
		resp, err := svc.UpdateCarrier(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "new@email.com", resp.NocEmail)
	})

	t.Run("update carrier without ID", func(t *testing.T) {
		req := &routingv1.UpdateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Name: "Test",
				Asn:  "9999",
			},
		}
		_, err := svc.UpdateCarrier(ctx, req)
		assert.Error(t, err)
	})

	t.Run("update non-existent carrier", func(t *testing.T) {
		req := &routingv1.UpdateCarrierRequest{
			Carrier: &routingv1.CarrierConfig{
				Id:   "non-existent",
				Name: "Test",
				Asn:  "9999",
			},
		}
		_, err := svc.UpdateCarrier(ctx, req)
		assert.Error(t, err)
	})
}

func TestCarrierService_DeleteCarrier(t *testing.T) {
	svc := setupCarrierService(t)
	ctx := context.Background()

	// Create a carrier first
	createReq := &routingv1.CreateCarrierRequest{
		Carrier: &routingv1.CarrierConfig{
			Name: "DeleteTest",
			Asn:  "3001",
		},
	}
	created, err := svc.CreateCarrier(ctx, createReq)
	require.NoError(t, err)

	t.Run("delete carrier", func(t *testing.T) {
		req := &routingv1.DeleteCarrierRequest{Id: created.Id}
		resp, err := svc.DeleteCarrier(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify it's gone
		_, err = svc.GetCarrier(ctx, &routingv1.GetCarrierRequest{Id: created.Id})
		assert.Error(t, err)
	})

	t.Run("delete carrier without ID", func(t *testing.T) {
		req := &routingv1.DeleteCarrierRequest{}
		_, err := svc.DeleteCarrier(ctx, req)
		assert.Error(t, err)
	})

	t.Run("delete non-existent carrier", func(t *testing.T) {
		req := &routingv1.DeleteCarrierRequest{Id: "non-existent"}
		_, err := svc.DeleteCarrier(ctx, req)
		assert.Error(t, err)
	})
}

func TestProtoConversion(t *testing.T) {
	t.Run("protoToCarrier with nil", func(t *testing.T) {
		result := protoToCarrier(nil)
		assert.Nil(t, result)
	})

	t.Run("carrierToProto with nil", func(t *testing.T) {
		result := carrierToProto(nil)
		assert.Nil(t, result)
	})

	t.Run("round trip conversion", func(t *testing.T) {
		original := &routingv1.CarrierConfig{
			Id:               "test-id",
			Name:             "TestCarrier",
			Asn:              "12345",
			NocEmail:         "noc@test.com",
			NocPhone:         "+1-555-1234",
			NocPortalUrl:     "https://noc.test.com",
			TeamId:           "team-123",
			AutoTicket:       true,
			TicketProviderId: "jira",
		}

		carrier := protoToCarrier(original)
		result := carrierToProto(carrier)

		assert.Equal(t, original.Id, result.Id)
		assert.Equal(t, original.Name, result.Name)
		assert.Equal(t, original.Asn, result.Asn)
		assert.Equal(t, original.NocEmail, result.NocEmail)
		assert.Equal(t, original.NocPhone, result.NocPhone)
		assert.Equal(t, original.NocPortalUrl, result.NocPortalUrl)
		assert.Equal(t, original.TeamId, result.TeamId)
		assert.Equal(t, original.AutoTicket, result.AutoTicket)
		assert.Equal(t, original.TicketProviderId, result.TicketProviderId)
	})
}
