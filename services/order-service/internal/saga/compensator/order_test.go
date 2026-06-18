package compensator_test

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/compensator"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderCompensator_Plan(t *testing.T) {
	t.Parallel()

	reserveStep := steps.NewReserveInventoryStep(nil)
	paymentStep := steps.NewProcessPaymentStep(nil)
	comp := compensator.NewOrderCompensator(reserveStep, paymentStep)

	cases := []struct {
		name       string
		failed     steps.Step
		wantNames  []string
		wantFirst  steps.Step
		wantSecond steps.Step
	}{
		{
			name:      "reserve failure compensates inventory",
			failed:    reserveStep,
			wantNames: []string{"inventory"},
			wantFirst: reserveStep,
		},
		{
			name:      "payment failure compensates inventory",
			failed:    paymentStep,
			wantNames: []string{"inventory"},
			wantFirst: reserveStep,
		},
		{
			name:       "confirm failure compensates payment then inventory",
			failed:     steps.NewConfirmOrderStep(nil),
			wantNames:  []string{"payment", "inventory"},
			wantFirst:  paymentStep,
			wantSecond: reserveStep,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := comp.Plan(&domain.Saga{}, tc.failed)
			require.Len(t, plan, len(tc.wantNames))
			for i, name := range tc.wantNames {
				assert.Equal(t, name, plan[i].Name())
			}
			assert.Equal(t, tc.wantFirst, plan[0])
			if tc.wantSecond != nil {
				assert.Equal(t, tc.wantSecond, plan[1])
			}
		})
	}
}
