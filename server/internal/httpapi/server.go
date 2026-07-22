package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cpa-monitor/server/internal/accounthealth"
	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/deployments"
	"cpa-monitor/server/internal/gateways"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/observability"
	"cpa-monitor/server/internal/subscriptions"
)

const maxUploadBytes = 2 << 20
const appVersion = "1.3.0"

type Server struct {
	settings       *config.Store
	monitor        *Monitor
	subs           *subscriptions.Manager
	control        *controlplane.Store
	deployments    *deployments.Coordinator
	collector      *observability.Collector
	poller         *accounthealth.Scheduler
	settingsUpdate func(config.Settings)
	preflightKey   []byte
	now            func() time.Time
}

func NewServer(settings *config.Store, monitor *Monitor, subs *subscriptions.Manager, control *controlplane.Store, coordinator *deployments.Coordinator, collector *observability.Collector) *Server {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("preflight key generation failed")
	}
	return &Server{settings: settings, monitor: monitor, subs: subs, control: control, deployments: coordinator, collector: collector, preflightKey: key, now: time.Now}
}

func (s *Server) Handler() http.Handler {
	return s.middleware(http.HandlerFunc(s.route))
}

func (s *Server) SetSettingsUpdateHandler(handler func(config.Settings)) {
	s.settingsUpdate = handler
}

func (s *Server) SetAccountPoller(poller *accounthealth.Scheduler) {
	s.poller = poller
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	switch {
	case path == "/api/health":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "name": "CPA Orbit", "version": appVersion, "time": time.Now()})
	case path == "/api/cpa/status":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		s.handleCPAStatus(w, r)
	case path == "/api/offers":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, s.monitor.Offers())
	case path == "/api/offers/refresh":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		offers, err := s.monitor.Refresh(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "refresh_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, offers)
	case path == "/api/gpt-plus":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, s.monitor.GPTPlusOffers())
	case path == "/api/gpt-plus/refresh":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if _, err := s.monitor.Refresh(r.Context()); err != nil {
			writeError(w, http.StatusBadGateway, "refresh_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s.monitor.GPTPlusOffers())
	case path == "/api/price-history":
		if !requireMethod(w, r, http.MethodDelete) {
			return
		}
		source := r.URL.Query().Get("source")
		at, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("at"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_history_time", "at must be an RFC3339 timestamp")
			return
		}
		deleted, err := s.monitor.DeletePriceSample(source, at)
		if errors.Is(err, ErrInvalidPriceHistorySource) {
			writeError(w, http.StatusBadRequest, "invalid_history_source", err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "history_delete_failed", err.Error())
			return
		}
		if !deleted {
			writeError(w, http.StatusNotFound, "history_sample_not_found", "price history sample was not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case path == "/api/luban/balance":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		s.handleLubanBalance(w, r)
	case path == "/api/luban/countries":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		s.handleLubanCountries(w, r)
	case path == "/api/luban/services":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		s.handleLubanServices(w, r)
	case path == "/api/luban/number":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.handleLubanNumber(w, r)
	case path == "/api/luban/sms":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		s.handleLubanSMS(w, r)
	case path == "/api/luban/release":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.handleLubanRelease(w, r)
	case path == "/api/luban/key":
		if !requireMethod(w, r, http.MethodPut) {
			return
		}
		s.handleLubanKey(w, r)
	case path == "/api/settings":
		s.handleSettings(w, r)
	case path == "/api/settings/test-webhook":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if err := s.monitor.TestWebhook(r.Context(), s.settings.Get().WebhookURL); err != nil {
			writeError(w, http.StatusBadGateway, "webhook_test_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case path == "/api/gateways/targets":
		s.handleGatewayTargets(w, r)
	case path == "/api/gateways/operations":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		operations, err := s.deployments.Operations(r.Context(), queryInt(r, "limit", 100))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "operations_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"operations": operations})
	case path == "/api/gateways/overview":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		s.handleGatewayOverview(w, r)
	case path == "/api/gateways/collect":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		if err := s.collector.Collect(ctx); err != nil {
			writeError(w, http.StatusBadGateway, "collection_failed", "some Sub2API telemetry could not be refreshed; the last valid snapshot was kept")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"collected": true, "at": time.Now()})
	case path == "/api/gateways/usage":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		targetID, err := strconv.ParseInt(r.URL.Query().Get("targetId"), 10, 64)
		if err != nil || targetID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_target", "targetId is required")
			return
		}
		days := queryInt(r, "days", 7)
		if days < 1 || days > 90 {
			days = 7
		}
		to := time.Now().UTC()
		buckets, err := s.control.ListUsageBuckets(r.Context(), targetID, to.Add(-time.Duration(days)*24*time.Hour), to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "usage_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"buckets": buckets, "snapshots": s.collector.Snapshots(), "from": to.Add(-time.Duration(days) * 24 * time.Hour), "to": to})
	case strings.HasPrefix(path, "/api/gateways/targets/"):
		s.handleGatewayTargetAction(w, r, path)
	case path == "/api/subscriptions/poll-status":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if s.poller == nil {
			writeError(w, http.StatusServiceUnavailable, "poller_unavailable", "account polling is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, s.poller.Status())
	case path == "/api/subscriptions/poll-now":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if s.poller == nil {
			writeError(w, http.StatusServiceUnavailable, "poller_unavailable", "account polling is unavailable")
			return
		}
		if err := s.poller.PollNow(); err != nil {
			if errors.Is(err, accounthealth.ErrAlreadyRunning) {
				writeError(w, http.StatusConflict, "poll_in_progress", err.Error())
				return
			}
			writeError(w, http.StatusServiceUnavailable, "poller_unavailable", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"started": true})
	case path == "/api/subscriptions":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		page := queryInt(r, "page", 1)
		pageSize := queryInt(r, "pageSize", 10)
		result := s.subs.Page(r.URL.Query().Get("folder"), r.URL.Query().Get("status"), r.URL.Query().Get("search"), page, pageSize)
		writeJSON(w, http.StatusOK, result)
	case path == "/api/subscriptions/import/preflight":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.handleImportPreflight(w, r)
	case path == "/api/subscriptions/import/commit":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.handleImportCommit(w, r)
	case path == "/api/subscriptions/import":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		writeError(w, http.StatusGone, "import_endpoint_migrated", "use /api/subscriptions/import/preflight then /api/subscriptions/import/commit")
	case strings.HasPrefix(path, "/api/subscriptions/"):
		s.handleSubscriptionAction(w, r, path)
	case path == "/api/alerts":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"alerts": s.monitor.Alerts()})
	case path == "/api/dashboard":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		s.handleDashboard(w)
	default:
		writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
	}
}

func (s *Server) lubanKey() (string, error) {
	key := strings.TrimSpace(s.settings.Get().LubanAPIKey)
	if key == "" {
		return "", errors.New("请先配置鲁班 API 密钥")
	}
	return key, nil
}

func (s *Server) handleLubanCountries(w http.ResponseWriter, r *http.Request) {
	key, err := s.lubanKey()
	if err != nil {
		writeError(w, http.StatusBadRequest, "luban_not_configured", err.Error())
		return
	}
	body, err := queryLubanAPI(r.Context(), key, "countries", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "luban_countries_failed", err.Error())
		return
	}
	var payload struct {
		Msg []struct {
			ID     string `json:"id"`
			NameEN string `json:"name_en"`
			NameCN string `json:"name_cn"`
			Code   string `json:"code"`
		} `json:"msg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadGateway, "luban_countries_invalid", "鲁班国家列表格式异常")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"countries": payload.Msg})
}

func (s *Server) handleLubanServices(w http.ResponseWriter, r *http.Request) {
	key, err := s.lubanKey()
	if err != nil {
		writeError(w, http.StatusBadRequest, "luban_not_configured", err.Error())
		return
	}
	page := queryInt(r, "page", 1)
	if page < 1 {
		page = 1
	}
	language := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("language")))
	if language != "zh" && language != "en" {
		language = "en"
	}
	body, err := queryLubanAPI(r.Context(), key, "List", map[string]string{
		"country":  strings.TrimSpace(r.URL.Query().Get("country")),
		"service":  strings.TrimSpace(r.URL.Query().Get("service")),
		"language": language,
		"page":     strconv.Itoa(page),
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "luban_services_failed", err.Error())
		return
	}
	var payload struct {
		Msg []struct {
			ServiceID     string  `json:"service_id"`
			CountryNameZH string  `json:"country_name_zh"`
			CountryNameEN string  `json:"country_name_en"`
			ServiceName   string  `json:"service_name"`
			Provider      string  `json:"provider"`
			Cost          float64 `json:"cost"`
		} `json:"msg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadGateway, "luban_services_invalid", "鲁班服务列表格式异常")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": payload.Msg, "page": page})
}

func (s *Server) handleLubanNumber(w http.ResponseWriter, r *http.Request) {
	key, err := s.lubanKey()
	if err != nil {
		writeError(w, http.StatusBadRequest, "luban_not_configured", err.Error())
		return
	}
	var input struct {
		ServiceID string `json:"serviceId"`
	}
	if err := decodeJSON(w, r, &input, 64<<10); err != nil || strings.TrimSpace(input.ServiceID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_luban_service", "请选择有效的国家服务")
		return
	}
	body, err := queryLubanAPI(r.Context(), key, "getNumber", map[string]string{"service_id": strings.TrimSpace(input.ServiceID)})
	if err != nil {
		writeError(w, http.StatusBadGateway, "luban_number_failed", err.Error())
		return
	}
	var raw struct {
		Number    string          `json:"number"`
		RequestID json.RawMessage `json:"request_id"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || strings.TrimSpace(raw.Number) == "" {
		writeError(w, http.StatusBadGateway, "luban_number_invalid", "鲁班号码响应格式异常")
		return
	}
	requestID := strings.Trim(string(raw.RequestID), `"`)
	if requestID == "" || requestID == "null" {
		writeError(w, http.StatusBadGateway, "luban_number_invalid", "鲁班未返回请求编号")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requestId": requestID, "number": raw.Number, "status": "waiting"})
}

func (s *Server) handleLubanSMS(w http.ResponseWriter, r *http.Request) {
	key, err := s.lubanKey()
	if err != nil {
		writeError(w, http.StatusBadRequest, "luban_not_configured", err.Error())
		return
	}
	requestID := strings.TrimSpace(r.URL.Query().Get("requestId"))
	if requestID == "" || len(requestID) > 64 {
		writeError(w, http.StatusBadRequest, "invalid_luban_request", "请求编号无效")
		return
	}
	body, err := queryLubanAPI(r.Context(), key, "getSms", map[string]string{"request_id": requestID})
	if err != nil {
		if strings.Contains(err.Error(), "wrong_status") || strings.Contains(err.Error(), "尚未") {
			writeJSON(w, http.StatusOK, map[string]any{"status": "waiting", "requestId": requestID})
			return
		}
		writeError(w, http.StatusBadGateway, "luban_sms_failed", err.Error())
		return
	}
	var payload struct {
		Msg     string `json:"msg"`
		SMSCode string `json:"sms_code"`
		SMSMsg  *struct {
			Number string `json:"number"`
		} `json:"sms_msg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadGateway, "luban_sms_invalid", "鲁班短信响应格式异常")
		return
	}
	if payload.Msg == "success" && strings.TrimSpace(payload.SMSCode) != "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "received", "requestId": requestID, "code": payload.SMSCode})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "waiting", "requestId": requestID, "number": func() string {
		if payload.SMSMsg != nil {
			return payload.SMSMsg.Number
		}
		return ""
	}()})
}

func (s *Server) handleLubanRelease(w http.ResponseWriter, r *http.Request) {
	key, err := s.lubanKey()
	if err != nil {
		writeError(w, http.StatusBadRequest, "luban_not_configured", err.Error())
		return
	}
	var input struct {
		RequestID string `json:"requestId"`
	}
	if err := decodeJSON(w, r, &input, 64<<10); err != nil || strings.TrimSpace(input.RequestID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_luban_request", "请求编号无效")
		return
	}
	if _, err := queryLubanAPI(r.Context(), key, "setStatus", map[string]string{"request_id": strings.TrimSpace(input.RequestID), "status": "reject"}); err != nil {
		writeError(w, http.StatusBadGateway, "luban_release_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"released": true, "requestId": strings.TrimSpace(input.RequestID)})
}

func (s *Server) handleLubanBalance(w http.ResponseWriter, r *http.Request) {
	key := s.settings.Get().LubanAPIKey
	if strings.TrimSpace(key) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	balance, err := queryLubanBalance(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusBadGateway, "luban_balance_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "balance": balance, "checkedAt": time.Now()})
}

func (s *Server) handleLubanKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		APIKey string `json:"apiKey"`
	}
	if err := decodeJSON(w, r, &input, 64<<10); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	current := s.settings.Get()
	current.LubanAPIKey = strings.TrimSpace(input.APIKey)
	if err := s.settings.Update(current); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_luban_key", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": current.LubanAPIKey != ""})
}

type settingsUpdate struct {
	Threshold            *float64 `json:"threshold"`
	RefreshMinutes       *int     `json:"refreshMinutes"`
	AccountPollMinutes   *int     `json:"accountPollMinutes"`
	WebhookURL           *string  `json:"webhookUrl"`
	BaseURL              *string  `json:"baseUrl"`
	APIKey               *string  `json:"apiKey"`
	AllowRemoteBaseURL   *bool    `json:"allowRemoteBaseUrl"`
	CPAAuthDir           *string  `json:"cpaAuthDir"`
	SyncToCPAAuthDir     *bool    `json:"syncToCpaAuthDir"`
	ThemeMode            *string  `json:"themeMode"`
	StartOnLogin         *bool    `json:"startOnLogin"`
	CloseToTray          *bool    `json:"closeToTray"`
	DesktopNotifications *bool    `json:"desktopNotifications"`
	FlashOnAlert         *bool    `json:"flashOnAlert"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.settings.Public())
	case http.MethodPut:
		var input settingsUpdate
		if err := decodeJSON(w, r, &input, 1<<20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		current := s.settings.Get()
		if input.Threshold != nil {
			current.Threshold = *input.Threshold
		}
		if input.RefreshMinutes != nil {
			current.RefreshMinutes = *input.RefreshMinutes
		}
		if input.AccountPollMinutes != nil {
			current.AccountPollMinutes = *input.AccountPollMinutes
		}
		if input.WebhookURL != nil {
			current.WebhookURL = *input.WebhookURL
		}
		if input.BaseURL != nil {
			current.BaseURL = *input.BaseURL
		}
		if input.APIKey != nil {
			current.APIKey = *input.APIKey
		}
		if input.AllowRemoteBaseURL != nil {
			current.AllowRemoteBaseURL = *input.AllowRemoteBaseURL
		}
		if input.CPAAuthDir != nil {
			current.CPAAuthDir = *input.CPAAuthDir
		}
		if input.SyncToCPAAuthDir != nil {
			current.SyncToCPAAuthDir = *input.SyncToCPAAuthDir
		}
		if input.ThemeMode != nil {
			current.ThemeMode = *input.ThemeMode
		}
		if input.StartOnLogin != nil {
			current.StartOnLogin = *input.StartOnLogin
		}
		if input.CloseToTray != nil {
			current.CloseToTray = *input.CloseToTray
		}
		if input.DesktopNotifications != nil {
			current.DesktopNotifications = *input.DesktopNotifications
		}
		if input.FlashOnAlert != nil {
			current.FlashOnAlert = *input.FlashOnAlert
		}
		if err := s.settings.Update(current); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_settings", err.Error())
			return
		}
		if s.settingsUpdate != nil {
			s.settingsUpdate(current)
		}
		s.monitor.ResetSchedule()
		if s.poller != nil {
			s.poller.ResetSchedule()
		}
		writeJSON(w, http.StatusOK, s.settings.Public())
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

type preflightClaims struct {
	Version, Digest, OperationID string
	Expires                      int64
}

func readImportFile(w http.ResponseWriter, r *http.Request) ([]byte, *multipart.FileHeader, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+64*1024)
	if err := r.ParseMultipartForm(maxUploadBytes + 64*1024); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "multipart upload exceeds 2 MB file limit")
		return nil, nil, false
	}
	if len(r.MultipartForm.Value) != 0 || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		writeError(w, http.StatusBadRequest, "file_only_required", "multipart request must contain exactly one file field")
		return nil, nil, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "multipart field 'file' is required")
		return nil, nil, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", "failed to read upload")
		return nil, nil, false
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "JSON file exceeds 2 MB")
		return nil, nil, false
	}
	return data, header, true
}

func (s *Server) handleImportPreflight(w http.ResponseWriter, r *http.Request) {
	data, _, ok := readImportFile(w, r)
	if !ok {
		return
	}
	analysis, err := subscriptions.AnalyzeAuthJSON(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_auth_json", err.Error())
		return
	}
	if _, duplicate, duplicateErr := s.subs.ExactDuplicate(data); duplicateErr != nil {
		writeError(w, http.StatusBadRequest, "duplicate_check_failed", "could not check existing archives")
		return
	} else if duplicate {
		analysis.Duplicate = subscriptions.AuthAnalysisState{State: "duplicate", ReasonCode: "exact_duplicate", Message: "相同 JSON 已归档"}
		analysis.Compatibility[gateways.KindCPA] = subscriptions.TargetCompatibility{ReasonCode: "exact_duplicate"}
		analysis.Compatibility[gateways.KindSub2API] = subscriptions.TargetCompatibility{ReasonCode: "exact_duplicate"}
	}
	if assignment, assignmentErr := s.control.CredentialAssignment(r.Context(), analysis.Identity.LogicalID); assignmentErr == nil && assignment.Status != "failed" && assignment.Status != "released" {
		analysis.Conflict = subscriptions.AuthAnalysisState{State: "conflict", ReasonCode: "active_assignment_exists", Message: "该凭证已有运行池分配"}
		analysis.Compatibility[gateways.KindCPA] = subscriptions.TargetCompatibility{ReasonCode: "active_assignment_exists"}
		analysis.Compatibility[gateways.KindSub2API] = subscriptions.TargetCompatibility{ReasonCode: "active_assignment_exists"}
	} else if assignmentErr != nil && !errors.Is(assignmentErr, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "assignment_check_failed", "could not check runtime assignments")
		return
	}
	targets, err := s.deployments.Targets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "targets_failed", "gateway targets unavailable")
		return
	}
	compatible := make([]map[string]any, 0)
	for _, target := range targets {
		compat := analysis.Compatibility[gateways.Kind(target.Kind)]
		compatible = append(compatible, map[string]any{"targetId": target.ID, "kind": target.Kind, "name": target.Name, "enabled": target.Enabled, "compatible": compat.Compatible, "reasonCode": compat.ReasonCode})
	}
	opBytes := make([]byte, 16)
	_, _ = rand.Read(opBytes)
	claims := preflightClaims{Version: analysis.Version, Digest: analysis.Digest, OperationID: hex.EncodeToString(opBytes), Expires: s.now().Add(5 * time.Minute).Unix()}
	token := s.signPreflight(claims)
	writeJSON(w, http.StatusOK, map[string]any{"operationId": claims.OperationID, "expiresAt": time.Unix(claims.Expires, 0).UTC(), "preflightToken": token, "analysis": analysis, "targets": compatible})
}

func (s *Server) handleImportCommit(w http.ResponseWriter, r *http.Request) {
	targetValues := r.URL.Query()["targetId"]
	if len(targetValues) != 1 {
		writeError(w, http.StatusBadRequest, "single_target_required", "exactly one targetId is required")
		return
	}
	targetID, err := strconv.ParseInt(targetValues[0], 10, 64)
	if err != nil || targetID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_target", "targetId must be a positive integer")
		return
	}
	claims, err := s.verifyPreflight(strings.TrimSpace(r.Header.Get("X-Orbit-Preflight-Token")))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_preflight_token", "preflight token is invalid or expired")
		return
	}
	data, header, ok := readImportFile(w, r)
	if !ok {
		return
	}
	analysis, err := subscriptions.AnalyzeAuthJSON(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_auth_json", err.Error())
		return
	}
	if analysis.Version != claims.Version || analysis.Digest != claims.Digest {
		writeError(w, http.StatusConflict, "preflight_digest_mismatch", "uploaded file does not match preflight")
		return
	}
	target, err := s.control.GatewayTarget(r.Context(), targetID)
	if err != nil || !target.Enabled {
		writeError(w, http.StatusBadRequest, "target_unavailable", "target is missing or disabled")
		return
	}
	compat := analysis.Compatibility[gateways.Kind(target.Kind)]
	if !compat.Compatible {
		writeError(w, http.StatusBadRequest, compat.ReasonCode, "authentication JSON is incompatible with the selected target")
		return
	}
	acquisitionPrice := strings.TrimSpace(r.URL.Query().Get("acquisitionPrice"))
	op, created, err := s.control.ReserveImportOperation(r.Context(), controlplane.ImportOperation{ID: claims.OperationID, Digest: claims.Digest, TargetID: targetID, Status: "running", AcquisitionPrice: acquisitionPrice})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import_reservation_failed", "import could not be reserved")
		return
	}
	if !created {
		if op.Digest != claims.Digest || op.TargetID != targetID || op.AcquisitionPrice != acquisitionPrice {
			writeError(w, http.StatusConflict, "operation_conflict", "preflight operation was already used differently")
			return
		}
		if op.Status == "succeeded" {
			sub, found := s.subs.Get(op.SubscriptionID)
			if !found {
				writeError(w, http.StatusConflict, "operation_incomplete", "import operation archive is unavailable")
				return
			}
			bindings, _ := s.deployments.Bindings(r.Context(), sub.ID)
			writeJSON(w, http.StatusOK, map[string]any{"operationId": claims.OperationID, "subscriptionId": sub.ID, "subscription": sub, "deployment": bindingForTarget(bindings, targetID), "outcome": "succeeded", "retryable": false, "httpStatus": http.StatusOK, "archived": true, "idempotent": true})
			return
		}
		if op.Status == "failed" && op.Retryable && op.SubscriptionID != "" {
			sub, found := s.subs.Get(op.SubscriptionID)
			if !found {
				writeError(w, http.StatusConflict, "operation_incomplete", "import operation archive is unavailable")
				return
			}
			if _, retryErr := s.control.RetryImportOperation(r.Context(), claims.OperationID); retryErr != nil {
				writeError(w, http.StatusConflict, "operation_not_retryable", "import operation cannot be retried")
				return
			}
			binding, deployErr := s.deployments.Deploy(r.Context(), sub.ID, targetID)
			if deployErr != nil {
				var safe *gateways.DeploymentError
				code, message, outcome, retryable, status := "deployment_failed", "subscription was archived but deployment failed", "failed", false, http.StatusBadGateway
				if errors.As(deployErr, &safe) {
					code, message, outcome, retryable, status = safe.Code, safe.Message, string(safe.Outcome), safe.Retryable, safe.HTTPStatus
					if status == 0 {
						status = http.StatusBadGateway
					}
				}
				_ = s.control.CompleteImportOperationOutcome(r.Context(), claims.OperationID, "failed", sub.ID, message, code, outcome, retryable, status)
				writeJSON(w, status, map[string]any{"operationId": claims.OperationID, "subscriptionId": sub.ID, "targetId": targetID, "outcome": outcome, "retryable": retryable, "httpStatus": status, "archived": true, "error": map[string]any{"code": code, "message": message}})
				return
			}
			_ = s.control.CompleteImportOperationOutcome(r.Context(), claims.OperationID, "succeeded", sub.ID, "", "", "", false, http.StatusOK)
			writeJSON(w, http.StatusOK, map[string]any{"operationId": claims.OperationID, "subscriptionId": sub.ID, "deployment": binding, "outcome": "succeeded", "retryable": false, "httpStatus": http.StatusOK, "archived": true, "idempotent": true})
			return
		}
		writeError(w, http.StatusConflict, "operation_in_progress", "import operation is already in progress")
		return
	}
	provider := string(gateways.KindSub2API)
	if gateways.Kind(target.Kind) == gateways.KindCPA {
		provider = string(gateways.KindCPA)
	}
	sub, _, err := s.subs.ImportWithOptions(data, header.Filename, subscriptions.ImportOptions{
		AcquisitionPrice: op.AcquisitionPrice,
		ArchiveProvider:  provider,
		SkipLegacySync:   true,
	})
	if err != nil {
		_ = s.control.CompleteImportOperationOutcome(r.Context(), claims.OperationID, "failed", "", "subscription could not be archived", "archive_failed", "failed", false, http.StatusBadRequest)
		writeError(w, http.StatusBadRequest, "import_failed", "subscription could not be archived")
		return
	}
	binding, err := s.deployments.Deploy(r.Context(), sub.ID, targetID)
	if err != nil {
		var safe *gateways.DeploymentError
		code, message, outcome, retryable, status := "deployment_failed", "subscription was archived but deployment failed", "failed", false, http.StatusBadGateway
		if errors.As(err, &safe) {
			code, message, outcome, retryable, status = safe.Code, safe.Message, string(safe.Outcome), safe.Retryable, safe.HTTPStatus
			if status == 0 {
				status = http.StatusBadGateway
			}
		}
		_ = s.control.CompleteImportOperationOutcome(r.Context(), claims.OperationID, "failed", sub.ID, message, code, outcome, retryable, status)
		writeJSON(w, status, map[string]any{"operationId": claims.OperationID, "subscriptionId": sub.ID, "targetId": targetID, "outcome": outcome, "retryable": retryable, "httpStatus": status, "archived": true, "error": map[string]any{"code": code, "message": message}})
		return
	}
	_ = s.control.CompleteImportOperationOutcome(r.Context(), claims.OperationID, "succeeded", sub.ID, "", "", "", false, http.StatusCreated)
	writeJSON(w, http.StatusCreated, map[string]any{"operationId": claims.OperationID, "subscriptionId": sub.ID, "subscription": sub, "deployment": binding, "outcome": "succeeded", "retryable": false, "httpStatus": http.StatusCreated, "archived": true, "idempotent": false})
}

func bindingForTarget(bindings []controlplane.DeploymentBinding, targetID int64) any {
	for _, binding := range bindings {
		if binding.TargetID == targetID {
			return binding
		}
	}
	return nil
}

func (s *Server) signPreflight(claims preflightClaims) string {
	payload, _ := json.Marshal(claims)
	encoded := hex.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.preflightKey)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil))
}
func (s *Server) verifyPreflight(token string) (preflightClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return preflightClaims{}, errors.New("invalid")
	}
	signature, err := hex.DecodeString(parts[1])
	if err != nil {
		return preflightClaims{}, errors.New("invalid")
	}
	mac := hmac.New(sha256.New, s.preflightKey)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return preflightClaims{}, errors.New("invalid")
	}
	payload, err := hex.DecodeString(parts[0])
	if err != nil {
		return preflightClaims{}, errors.New("invalid")
	}
	var claims preflightClaims
	if json.Unmarshal(payload, &claims) != nil || claims.Version != subscriptions.AuthAnalysisVersion || claims.Digest == "" || claims.OperationID == "" || s.now().Unix() > claims.Expires {
		return preflightClaims{}, errors.New("invalid")
	}
	return claims, nil
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+64*1024)
	if err := r.ParseMultipartForm(maxUploadBytes + 64*1024); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "multipart upload exceeds 2 MB file limit")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "multipart field 'file' is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", "failed to read upload")
		return
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "JSON file exceeds 2 MB")
		return
	}
	acquisitionPrice := strings.TrimSpace(r.FormValue("acquisitionPrice"))
	if acquisitionPrice == "" {
		acquisitionPrice = strings.TrimSpace(r.URL.Query().Get("acquisitionPrice"))
	}
	deployToGateway, _ := strconv.ParseBool(strings.TrimSpace(r.URL.Query().Get("deploy")))
	archiveProvider := "cpa"
	if deployToGateway {
		archiveProvider = "sub2api"
	}
	sub, synced, err := s.subs.ImportWithOptions(data, header.Filename, subscriptions.ImportOptions{AcquisitionPrice: acquisitionPrice, ArchiveProvider: archiveProvider, SkipLegacySync: deployToGateway})
	if errors.Is(err, subscriptions.ErrDuplicateSubscription) {
		message := "订阅未导入：相同账号已经存在"
		var duplicate *subscriptions.DuplicateSubscriptionError
		if errors.As(err, &duplicate) {
			target := strings.TrimSpace(duplicate.ExistingEmail)
			if duplicate.ExistingFile != "" {
				if target != "" {
					target += "（" + duplicate.ExistingFile + "）"
				} else {
					target = duplicate.ExistingFile
				}
			}
			switch duplicate.MatchField {
			case "json":
				message = "订阅未导入：完整 JSON 内容已存在"
			case "email":
				message = "订阅未导入：邮箱已存在"
			case "account_id":
				message = "订阅未导入：账号 ID 已存在"
			case "cpa_auth":
				message = "订阅未导入：CPA 运行池中已存在相同账号"
			}
			if target != "" {
				message += "，现有订阅为 " + target
			}
		}
		writeError(w, http.StatusConflict, "duplicate_subscription", message)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "import_failed", err.Error())
		return
	}
	result := map[string]any{"subscription": sub, "syncedToCpa": synced}
	if deployToGateway {
		binding, deployErr := s.deployments.DeployDefault(r.Context(), sub.ID)
		if deployErr != nil {
			result["deploymentError"] = deployErr.Error()
		} else {
			result["deployment"] = binding
		}
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleSubscriptionAction(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/subscriptions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not_found", "subscription action not found")
		return
	}
	id, err := url.PathUnescape(parts[0])
	if err != nil || strings.ContainsAny(id, `/\\`) {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid subscription id")
		return
	}
	if len(parts) == 1 {
		if !requireMethod(w, r, http.MethodDelete) {
			return
		}
		if err := s.deployments.DetachAll(r.Context(), id); err != nil {
			writeError(w, http.StatusBadGateway, "detach_failed", "runtime bindings could not be removed; the subscription archive was kept")
			return
		}
		if err := s.subs.Delete(id); errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "subscription not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		return
	}
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not_found", "subscription action not found")
		return
	}
	if parts[1] == "bindings" {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		bindings, err := s.deployments.Bindings(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "bindings_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"bindings": bindings})
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	switch parts[1] {
	case "test":
		result, err := s.deployments.Inspect(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			result = model.Connectivity{Status: "pending", ReasonCode: "not_deployed", Error: "subscription is not deployed to a gateway", CheckedAt: time.Now()}
			err = nil
		}
		if err == nil {
			err = s.subs.SaveConnectivity(id, result)
		}
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "test_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "sync":
		binding, err := s.deployments.DeployDefault(r.Context(), id)
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "sync_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"syncedToCpa": binding.TargetID != 0, "fileName": filepath.Base(binding.RemoteAccountID), "deployment": binding})
	case "deploy":
		var input struct {
			TargetID int64 `json:"targetId"`
		}
		if r.ContentLength != 0 {
			if err := decodeJSON(w, r, &input, 64<<10); err != nil {
				return
			}
		}
		var binding controlplane.DeploymentBinding
		var err error
		if input.TargetID == 0 {
			binding, err = s.deployments.DeployDefault(r.Context(), id)
		} else {
			binding, err = s.deployments.Deploy(r.Context(), id, input.TargetID)
		}
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadGateway, "deployment_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, binding)
	case "detach":
		var input struct {
			TargetID int64 `json:"targetId"`
		}
		if err := decodeJSON(w, r, &input, 64<<10); err != nil {
			return
		}
		if input.TargetID == 0 {
			writeError(w, http.StatusBadRequest, "invalid_target", "targetId is required")
			return
		}
		binding, err := s.deployments.Detach(r.Context(), id, input.TargetID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "detach_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, binding)
	case "migrate":
		var input struct {
			FromTargetID int64 `json:"fromTargetId"`
			ToTargetID   int64 `json:"toTargetId"`
		}
		if err := decodeJSON(w, r, &input, 64<<10); err != nil {
			return
		}
		if input.FromTargetID == 0 || input.ToTargetID == 0 {
			writeError(w, http.StatusBadRequest, "invalid_migration", "fromTargetId and toTargetId are required")
			return
		}
		binding, err := s.deployments.Migrate(r.Context(), id, input.FromTargetID, input.ToTargetID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "migration_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, binding)
	default:
		writeError(w, http.StatusNotFound, "not_found", "subscription action not found")
	}
}

type gatewayTargetInput struct {
	ID                 int64   `json:"id"`
	Kind               string  `json:"kind"`
	Name               string  `json:"name"`
	BaseURL            string  `json:"baseUrl"`
	AdminKey           string  `json:"adminKey"`
	Enabled            bool    `json:"enabled"`
	Primary            bool    `json:"primary"`
	AllowRemote        bool    `json:"allowRemote"`
	DefaultGroupIDs    []int64 `json:"defaultGroupIds"`
	DefaultConcurrency int     `json:"defaultConcurrency"`
	DefaultPriority    int     `json:"defaultPriority"`
	RateMultiplier     float64 `json:"rateMultiplier"`
}

func (s *Server) handleGatewayTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		targets, err := s.deployments.Targets(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "targets_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"targets": targets})
	case http.MethodPost, http.MethodPut:
		var input gatewayTargetInput
		if err := decodeJSON(w, r, &input, 256<<10); err != nil {
			return
		}
		target, err := s.deployments.UpsertTarget(r.Context(), controlplane.GatewayTarget{
			ID: input.ID, Kind: input.Kind, Name: input.Name, BaseURL: input.BaseURL, AdminKey: input.AdminKey,
			Enabled: input.Enabled, Primary: input.Primary, AllowRemote: input.AllowRemote,
			DefaultGroupIDs: input.DefaultGroupIDs, DefaultConcurrency: input.DefaultConcurrency,
			DefaultPriority: input.DefaultPriority, RateMultiplier: input.RateMultiplier,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_gateway_target", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, target)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost, http.MethodPut)
	}
}

func (s *Server) handleGatewayTargetAction(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/gateways/targets/"), "/")
	if len(parts) != 2 || parts[1] != "test" {
		writeError(w, http.StatusNotFound, "not_found", "gateway target action not found")
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	targetID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || targetID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_target", "invalid gateway target ID")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	health, err := s.deployments.TargetHealth(ctx, targetID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "gateway_test_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) handleGatewayOverview(w http.ResponseWriter, r *http.Request) {
	targets, err := s.deployments.Targets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "overview_failed", err.Error())
		return
	}
	statuses := make([]map[string]any, 0, len(targets))
	for _, target := range targets {
		entry := map[string]any{"target": target, "health": map[string]any{"status": "disabled"}}
		if target.Enabled {
			ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
			health, healthErr := s.deployments.TargetHealth(ctx, target.ID)
			cancel()
			if healthErr != nil {
				entry["health"] = map[string]any{"status": "unavailable", "message": healthErr.Error(), "checkedAt": time.Now()}
			} else {
				entry["health"] = health
			}
		}
		statuses = append(statuses, entry)
	}
	bindings, _ := s.deployments.Bindings(r.Context(), "")
	operations, _ := s.deployments.Operations(r.Context(), 20)
	if bindings == nil {
		bindings = make([]controlplane.DeploymentBinding, 0)
	}
	if operations == nil {
		operations = make([]controlplane.SyncOperation, 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": statuses, "bindings": bindings, "operations": operations, "snapshots": s.collector.Snapshots(), "checkedAt": time.Now()})
}

func (s *Server) handleCPAStatus(w http.ResponseWriter, r *http.Request) {
	settings := s.settings.Get()
	result := map[string]any{
		"online": false, "baseUrl": settings.BaseURL, "authFileCount": 0,
		"version": "7.2.71", "checkedAt": time.Now(),
	}
	if entries, err := os.ReadDir(settings.CPAAuthDir); err == nil {
		count := 0
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
				continue
			}
			if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
				count++
			}
		}
		result["authFileCount"] = count
	} else {
		result["error"] = "CPA auth-dir is unavailable"
	}
	if err := config.ValidateBaseURL(settings.BaseURL, settings.AllowRemoteBaseURL); err != nil {
		result["error"] = err.Error()
		writeJSON(w, http.StatusOK, result)
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		baseURL = baseURL[:len(baseURL)-3]
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(baseURL, "/")+"/healthz", nil)
	if err != nil {
		result["error"] = "failed to build CPA health request"
		writeJSON(w, http.StatusOK, result)
		return
	}
	started := time.Now()
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	result["latencyMs"] = time.Since(started).Milliseconds()
	if err != nil {
		result["error"] = err.Error()
		writeJSON(w, http.StatusOK, result)
		return
	}
	resp.Body.Close()
	result["httpStatus"] = resp.StatusCode
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result["online"] = true
	} else {
		result["error"] = fmt.Sprintf("health endpoint returned HTTP %d", resp.StatusCode)
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDashboard(w http.ResponseWriter) {
	offers := s.monitor.Offers()
	gptPlus := s.monitor.GPTPlusOffers()
	alerts := s.monitor.Alerts()
	items := s.subs.List("", "", "")
	connected, expired, unchecked, actionRequired := 0, 0, 0, 0
	for _, item := range items {
		connectivityStatus := item.Connectivity.Status
		isExpired := item.Status == "expired"
		if connectivityStatus == "ok" {
			connected++
		}
		if isExpired {
			expired++
		}
		if connectivityStatus == "unknown" || connectivityStatus == "" {
			unchecked++
		}
		if isExpired || connectivityStatus != "ok" {
			actionRequired++
		}
	}
	var lowestPrice *float64
	if len(offers.Offers) > 0 {
		value := offers.Offers[0].Price
		lowestPrice = &value
	}
	if len(gptPlus.Offers) > 0 && (lowestPrice == nil || gptPlus.Offers[0].Price < *lowestPrice) {
		value := gptPlus.Offers[0].Price
		lowestPrice = &value
	}
	stats := map[string]any{
		"totalSubscriptions": len(items), "connected": connected, "expired": expired,
		"unchecked": unchecked, "actionRequired": actionRequired, "lowestPrice": lowestPrice,
		"offersUpdatedAt": offers.UpdatedAt, "threshold": s.settings.Get().Threshold,
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"offers": offers.Offers, "gptPlusOffers": gptPlus.Offers, "priceHistory": s.monitor.PriceHistory(), "gptPlusPriceHistory": s.monitor.GPTPlusPriceHistory(), "subscriptions": items, "alerts": alerts, "stats": stats,
	})
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !allowedOrigin(origin) {
				writeError(w, http.StatusForbidden, "cors_forbidden", "origin is not allowed")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Orbit-Preflight-Token")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	return host == "localhost" || host == "127.0.0.1"
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(key)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func requireMethod(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, method := range methods {
		if r.Method == method {
			return true
		}
	}
	methodNotAllowed(w, methods...)
	return false
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}, "status": strconv.Itoa(status)})
}
