package gateways

import (
	"context"
	"time"

	"cpa-monitor/server/internal/model"
)

// Kind identifies a supported runtime gateway implementation.
type Kind string

const (
	KindCPA     Kind = "cpa"
	KindSub2API Kind = "sub2api"
)

// Credential is the control-plane representation of an archived subscription.
// Data is intentionally kept opaque so adapters can support provider-specific JSON.
type Credential struct {
	SubscriptionID   string
	Data             []byte
	Email            string
	AccountID        string
	ChatGPTAccountID string
	Provider         string
}

// BindingRef points at the runtime resource created for one subscription.
type BindingRef struct {
	TargetID    string `json:"targetId"`
	ExternalID  string `json:"externalId"`
	ExternalRef string `json:"externalRef,omitempty"`
}

type DeployOptions struct {
	UpdateExisting bool
	GroupIDs       []int64
}

type DeploymentResult struct {
	Binding BindingRef
	Status  string
	Message string
}

type Health struct {
	Status    string
	LatencyMS int64
	CheckedAt time.Time
	Message   string
}

type InspectResult struct {
	Connectivity model.Connectivity
}

// Adapter is the narrow contract used by deployment orchestration. Adapters own
// gateway protocol details; the subscription archive remains gateway-neutral.
type Adapter interface {
	Kind() Kind
	Health(context.Context) (Health, error)
	Deploy(context.Context, Credential, DeployOptions) (DeploymentResult, error)
	Inspect(context.Context, BindingRef, Credential) (InspectResult, error)
	Detach(context.Context, BindingRef, Credential) error
}
