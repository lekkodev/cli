package lekkoduration

import (
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

type RolloutConfig struct {
	ChanBufferSize    int64
	LockTtl           *durationpb.Duration
	NumRolloutWorkers int64
	RolloutTimeout    *durationpb.Duration
	Delay             *durationpb.Duration
	Jitter            *durationpb.Duration
}

// Rollout handler configuration
func getRollout(env string) *RolloutConfig {
	if env == "staging" {
		return &RolloutConfig{
			ChanBufferSize:    100,
			Delay:             &durationpb.Duration{Seconds: 180},
			Jitter:            &durationpb.Duration{Seconds: 30},
			LockTtl:           &durationpb.Duration{Seconds: 60},
			NumRolloutWorkers: 250,
			RolloutTimeout:    &durationpb.Duration{Seconds: 60},
		}
	}
	return &RolloutConfig{
		ChanBufferSize:    100,
		Delay:             &durationpb.Duration{Seconds: 900},
		Jitter:            &durationpb.Duration{Seconds: 60},
		LockTtl:           &durationpb.Duration{Seconds: 300},
		NumRolloutWorkers: 250,
		RolloutTimeout:    &durationpb.Duration{Seconds: 480},
	}
}

// getXyDuration
func getXyDuration() *durationpb.Duration {
	return &durationpb.Duration{Seconds: 480}
}
