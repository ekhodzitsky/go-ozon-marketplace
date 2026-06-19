package featureflags

import (
	"context"
	"hash/fnv"

	"github.com/open-feature/go-sdk/openfeature"
)

// Provider is an OpenFeature provider backed by RedisStore.
type Provider struct {
	store *RedisStore
}

// NewProvider creates a new OpenFeature provider from a Redis store.
func NewProvider(store *RedisStore) *Provider {
	return &Provider{store: store}
}

// Metadata returns provider metadata.
func (p *Provider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: "redis-featureflags"}
}

// Hooks returns hooks attached to the provider.
func (p *Provider) Hooks() []openfeature.Hook {
	return nil
}

// Init initializes the provider.
func (p *Provider) Init(_ openfeature.EvaluationContext) error {
	return nil
}

// Shutdown cleans up the provider.
func (p *Provider) Shutdown() {}

func (p *Provider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, flatCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	f, err := p.store.Get(ctx, flag)
	if err != nil {
		return openfeature.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewGeneralResolutionError(err.Error()),
				Reason:          openfeature.ErrorReason,
			},
		}
	}
	if f == nil {
		return openfeature.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewFlagNotFoundResolutionError(flag),
				Reason:          openfeature.ErrorReason,
			},
		}
	}
	if !f.Enabled {
		return openfeature.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				Reason: openfeature.DisabledReason,
			},
		}
	}

	userID, _ := flatCtx["targetingKey"].(string)
	value := evaluate(f, userID)
	return openfeature.BoolResolutionDetail{
		Value: value,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			Reason: openfeature.TargetingMatchReason,
		},
	}
}

func (p *Provider) StringEvaluation(ctx context.Context, flag string, defaultValue string, flatCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	return openfeature.StringResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			ResolutionError: openfeature.NewTypeMismatchResolutionError(flag),
			Reason:          openfeature.ErrorReason,
		},
	}
}

func (p *Provider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, flatCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	return openfeature.FloatResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			ResolutionError: openfeature.NewTypeMismatchResolutionError(flag),
			Reason:          openfeature.ErrorReason,
		},
	}
}

func (p *Provider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, flatCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	return openfeature.IntResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			ResolutionError: openfeature.NewTypeMismatchResolutionError(flag),
			Reason:          openfeature.ErrorReason,
		},
	}
}

func (p *Provider) ObjectEvaluation(ctx context.Context, flag string, defaultValue any, flatCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	return openfeature.InterfaceResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			ResolutionError: openfeature.NewTypeMismatchResolutionError(flag),
			Reason:          openfeature.ErrorReason,
		},
	}
}

func evaluate(f *Flag, userID string) bool {
	switch f.Strategy {
	case "percentage":
		if userID == "" {
			return false
		}
		h := hashUserID(userID + f.Name)
		return h%100 < uint32(f.Percentage)
	case "user_id":
		return userID != ""
	default:
		return true
	}
}

func hashUserID(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
