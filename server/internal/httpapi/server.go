package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/subscriptions"
)

const maxUploadBytes = 2 << 20
const appVersion = "1.0.2"

type Server struct {
	settings       *config.Store
	monitor        *Monitor
	subs           *subscriptions.Manager
	settingsUpdate func(config.Settings)
}

func NewServer(settings *config.Store, monitor *Monitor, subs *subscriptions.Manager) *Server {
	return &Server{settings: settings, monitor: monitor, subs: subs}
}

func (s *Server) Handler() http.Handler {
	return s.middleware(http.HandlerFunc(s.route))
}

func (s *Server) SetSettingsUpdateHandler(handler func(config.Settings)) {
	s.settingsUpdate = handler
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
	case path == "/api/subscriptions":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		page := queryInt(r, "page", 1)
		pageSize := queryInt(r, "pageSize", 10)
		result := s.subs.Page(r.URL.Query().Get("folder"), r.URL.Query().Get("status"), r.URL.Query().Get("search"), page, pageSize)
		writeJSON(w, http.StatusOK, result)
	case path == "/api/subscriptions/import":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.handleImport(w, r)
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
		writeJSON(w, http.StatusOK, s.settings.Public())
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
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
	sub, synced, err := s.subs.ImportWithOptions(data, header.Filename, subscriptions.ImportOptions{AcquisitionPrice: r.FormValue("acquisitionPrice")})
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
	writeJSON(w, http.StatusCreated, map[string]any{"subscription": sub, "syncedToCpa": synced})
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
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	switch parts[1] {
	case "test":
		result, err := s.subs.Test(r.Context(), id)
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
		target, err := s.subs.Sync(id)
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "sync_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"syncedToCpa": true, "fileName": filepath.Base(target)})
	default:
		writeError(w, http.StatusNotFound, "not_found", "subscription action not found")
	}
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
	stats := map[string]any{
		"totalSubscriptions": len(items), "connected": connected, "expired": expired,
		"unchecked": unchecked, "actionRequired": actionRequired, "lowestPrice": lowestPrice,
		"offersUpdatedAt": offers.UpdatedAt, "threshold": s.settings.Get().Threshold,
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"offers": offers.Offers, "priceHistory": s.monitor.PriceHistory(), "gptPlusPriceHistory": s.monitor.GPTPlusPriceHistory(), "subscriptions": items, "alerts": alerts, "stats": stats,
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
