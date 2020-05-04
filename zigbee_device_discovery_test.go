package zda

import (
	"context"
	"errors"
	"github.com/shimmeringbee/da"
	. "github.com/shimmeringbee/da/capabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

func TestZigbeeDeviceDiscovery_Contract(t *testing.T) {
	t.Run("can be assigned to a capability.DeviceDiscovery", func(t *testing.T) {
		assert.Implements(t, (*DeviceDiscovery)(nil), new(ZigbeeDeviceDiscovery))
	})
}

func TestZigbeeGateway_ReturnsDeviceDiscoveryCapability(t *testing.T) {
	t.Run("returns capability on query", func(t *testing.T) {
		zgw, _, stop := NewTestZigbeeGateway()
		defer stop(t)

		actualZdd := zgw.Capability(DeviceDiscoveryFlag)
		assert.IsType(t, (*ZigbeeDeviceDiscovery)(nil), actualZdd)
	})
}

func TestZigbeeDeviceDiscovery_Allow(t *testing.T) {
	t.Run("calling allow on device which is not the gateway self errors", func(t *testing.T) {
		zgw, _, stop := NewTestZigbeeGateway()
		defer stop(t)

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}
		nonSelfDevice := da.Device{}

		err := zdd.Allow(context.Background(), nonSelfDevice, 500*time.Millisecond)
		assert.Error(t, err)
	})

	t.Run("calling allow on device which is self causes AllowJoin of zigbee provider", func(t *testing.T) {
		zgw, provider, stop := NewTestZigbeeGateway()
		defer stop(t)

		provider.On("PermitJoin", mock.Anything, true).Return(nil)

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}

		err := zdd.Allow(context.Background(), zdd.gateway.Self(), 500*time.Millisecond)
		assert.NoError(t, err)

		status, err := zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.True(t, status.Discovering)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		event, err := zgw.ReadEvent(ctx)
		assert.NoError(t, err)
		assert.IsType(t, DeviceDiscoveryAllowed{}, event)
	})

	t.Run("calling allow on device which is self causes AllowJoin of zigbee provider, and forwards an error", func(t *testing.T) {
		zgw, provider, stop := NewTestZigbeeGateway()
		defer stop(t)

		expectedError := errors.New("permit join failure")
		provider.On("PermitJoin", mock.Anything, true).Return(expectedError)

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}

		err := zdd.Allow(context.Background(), zdd.gateway.Self(), 500*time.Millisecond)
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)

		status, err := zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.False(t, status.Discovering)
	})
}

func TestZigbeeDeviceDiscovery_Deny(t *testing.T) {
	t.Run("calling deny on device which is not the gateway self errors", func(t *testing.T) {
		zgw, _, stop := NewTestZigbeeGateway()
		defer stop(t)

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}
		nonSelfDevice := da.Device{}

		err := zdd.Deny(context.Background(), nonSelfDevice)

		assert.Error(t, err)
	})

	t.Run("calling deny on device which is self causes DenyJoin of zigbee provider", func(t *testing.T) {
		zgw, provider, stop := NewTestZigbeeGateway()
		defer stop(t)

		provider.On("DenyJoin", mock.Anything).Return(nil)

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}
		zdd.discovering = true

		err := zdd.Deny(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)

		status, err := zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.False(t, status.Discovering)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		event, err := zgw.ReadEvent(ctx)
		assert.NoError(t, err)
		assert.IsType(t, DeviceDiscoveryDenied{}, event)
	})

	t.Run("calling deny on device which is self causes DenyJoin of zigbee provider, and forwards an error", func(t *testing.T) {
		zgw, provider, stop := NewTestZigbeeGateway()
		defer stop(t)

		expectedError := errors.New("deny join failure")
		provider.On("DenyJoin", mock.Anything).Return(expectedError)

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}
		zdd.discovering = true

		err := zdd.Deny(context.Background(), zdd.gateway.Self())

		assert.Error(t, err)
		assert.Equal(t, expectedError, err)

		status, err := zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.True(t, status.Discovering)
	})
}

func TestZigbeeDeviceDiscovery_DurationBehaviour(t *testing.T) {
	t.Run("when an allows duration expires the a deny instruction is sent", func(t *testing.T) {
		zgw, provider, stop := NewTestZigbeeGateway()
		defer stop(t)

		provider.On("PermitJoin", mock.Anything, true).Return(nil)
		provider.On("DenyJoin", mock.Anything).Return(nil)

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}

		err := zdd.Allow(context.Background(), zdd.gateway.Self(), 100*time.Millisecond)
		assert.NoError(t, err)

		status, err := zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.True(t, status.Discovering)

		time.Sleep(150 * time.Millisecond)

		status, err = zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.False(t, status.Discovering)
	})

	t.Run("second allows extend the duration of the first", func(t *testing.T) {
		zgw, provider, stop := NewTestZigbeeGateway()
		defer stop(t)

		provider.On("PermitJoin", mock.Anything, true).Return(nil).Twice()

		zdd := ZigbeeDeviceDiscovery{gateway: zgw}

		err := zdd.Allow(context.Background(), zdd.gateway.Self(), 50*time.Millisecond)
		assert.NoError(t, err)

		status, err := zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.True(t, status.Discovering)
		assert.Greater(t, int64(status.RemainingDuration), int64(45*time.Millisecond))

		err = zdd.Allow(context.Background(), zdd.gateway.Self(), 200*time.Millisecond)
		assert.NoError(t, err)

		status, err = zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.True(t, status.Discovering)
		assert.Greater(t, int64(status.RemainingDuration), int64(145*time.Millisecond))

		time.Sleep(150 * time.Millisecond)

		status, err = zdd.Status(context.Background(), zdd.gateway.Self())
		assert.NoError(t, err)
		assert.True(t, status.Discovering)
	})
}
