package gateways

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"cpa-monitor/server/internal/model"
)

// IsSub2APIDataPackage recognizes both current typed exports and older
// markerless exports. The markerless form is intentionally identified by its
// complete envelope shape so an ordinary credential containing an accounts
// property cannot be routed to the wrong gateway.
func IsSub2APIDataPackage(data []byte) bool {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return false
	}
	var kind string
	if rawType, ok := root["type"]; ok {
		_ = json.Unmarshal(rawType, &kind)
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "sub2api-data" || kind == "sub2api-bundle" {
		return true
	}
	if kind != "" {
		return false
	}
	if _, ok := root["exported_at"]; !ok {
		return false
	}
	rawAccounts, hasAccounts := root["accounts"]
	rawProxies, hasProxies := root["proxies"]
	if !hasAccounts || !hasProxies {
		return false
	}
	var proxies []json.RawMessage
	if err := json.Unmarshal(rawProxies, &proxies); err != nil {
		return false
	}
	var accounts []struct {
		Platform    string          `json:"platform"`
		Type        string          `json:"type"`
		Credentials json.RawMessage `json:"credentials"`
	}
	if err := json.Unmarshal(rawAccounts, &accounts); err != nil {
		return false
	}
	for _, account := range accounts {
		var credentials map[string]any
		if strings.TrimSpace(account.Platform) == "" || strings.TrimSpace(account.Type) == "" || json.Unmarshal(account.Credentials, &credentials) != nil || len(credentials) == 0 {
			return false
		}
	}
	return true
}

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
	ChatGPTUserID    string
	AgentRuntimeID   string
	Provider         string
	CredentialSet    map[string]string
	LogicalIdentity  string
}

// BindingRef points at the runtime resource created for one subscription.
type BindingRef struct {
	TargetID    string `json:"targetId"`
	ExternalID  string `json:"externalId"`
	ExternalRef string `json:"externalRef,omitempty"`
	Managed     bool   `json:"managed"`
}

type DeployOptions struct {
	UpdateExisting bool
	GroupIDs       []int64
	Name           string
	Notes          string
	Concurrency    int
	Priority       int
	RateMultiplier float64
}

type DeploymentOutcome string

const (
	DeploymentFailed    DeploymentOutcome = "failed"
	DeploymentUncertain DeploymentOutcome = "uncertain"
)

// DeploymentError is safe to expose to API callers. Adapters must never put
// upstream response bodies, credentials, or management keys in Message.
type DeploymentError struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Outcome    DeploymentOutcome `json:"outcome"`
	Retryable  bool              `json:"retryable"`
	HTTPStatus int               `json:"httpStatus,omitempty"`
}

func (e *DeploymentError) Error() string {
	if e == nil {
		return "deployment failed"
	}
	return e.Message
}

type DeploymentResult struct {
	Binding        BindingRef `json:"binding"`
	Status         string     `json:"status"`
	Message        string     `json:"message,omitempty"`
	OperationID    string     `json:"operationId,omitempty"`
	SubscriptionID string     `json:"subscriptionId,omitempty"`
}

type Health struct {
	Status    string    `json:"status"`
	LatencyMS int64     `json:"latencyMs"`
	CheckedAt time.Time `json:"checkedAt"`
	Message   string    `json:"message,omitempty"`
}

type InspectResult struct {
	Connectivity model.Connectivity `json:"connectivity"`
}

type BindingReconciliation struct {
	Outcome string
	Binding BindingRef
}

type BindingReconciler interface {
	ReconcileBinding(context.Context, BindingRef, Credential) (BindingReconciliation, error)
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
