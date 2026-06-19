package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
)

func FeatureFlags(ctx context.Context, flags *featureflags.FeatureFlags, userID string) (*model.FeatureFlags, error) {
	return &model.FeatureFlags{
		NewCheckoutFlow: flags.IsEnabled(ctx, "new-checkout-flow", userID),
		FastSearch:      flags.IsEnabled(ctx, "fast-search", userID),
		DiscountSystem:  flags.IsEnabled(ctx, "discount-system", userID),
		RealTimeUpdates: flags.IsEnabled(ctx, "real-time-updates", userID),
	}, nil
}

func AbTestAssignments(_ context.Context, experiments []*abtesting.Experiment, userID string) ([]*model.ABTestAssignment, error) {
	assignments := make([]*model.ABTestAssignment, 0, len(experiments))
	for _, exp := range experiments {
		assignments = append(assignments, &model.ABTestAssignment{
			Experiment: exp.Name,
			Variation:  exp.Assign(userID),
		})
	}
	return assignments, nil
}
