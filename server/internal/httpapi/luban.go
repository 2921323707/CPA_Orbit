package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const lubanAPIBaseURL = "https://lubansms.com/v2/api/"
const lubanBalanceURL = lubanAPIBaseURL + "getBalance"

type lubanBalancePayload struct {
	Code    int             `json:"code"`
	Message string          `json:"msg"`
	Balance json.RawMessage `json:"balance"`
}

type lubanGenericPayload struct {
	Code int             `json:"code"`
	Msg  json.RawMessage `json:"msg"`
}

func queryLubanAPI(ctx context.Context, apiKey, endpoint string, params map[string]string) (json.RawMessage, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("Luban API key is not configured")
	}
	base, err := url.Parse(lubanAPIBaseURL + strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return nil, errors.New("failed to build Luban request")
	}
	query := base.Query()
	query.Set("apikey", apiKey)
	for key, value := range params {
		if strings.TrimSpace(value) != "" {
			query.Set(key, value)
		}
	}
	base.RawQuery = query.Encode()
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, errors.New("failed to build Luban request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CPA-Monitor/1.0")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Luban request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Luban endpoint returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, errors.New("failed to read Luban response")
	}
	var payload lubanGenericPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("invalid Luban response")
	}
	if payload.Code != 0 {
		message := strings.TrimSpace(string(payload.Msg))
		var text string
		if json.Unmarshal(payload.Msg, &text) == nil {
			message = strings.TrimSpace(text)
		}
		message = strings.Trim(message, `"`)
		if message == "" {
			message = "Luban request was rejected"
		}
		return nil, errors.New(message)
	}
	return json.RawMessage(body), nil
}

func queryLubanBalance(ctx context.Context, apiKey string) (float64, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return 0, errors.New("Luban API key is not configured")
	}
	endpoint, _ := url.Parse(lubanBalanceURL)
	query := endpoint.Query()
	query.Set("apikey", apiKey)
	endpoint.RawQuery = query.Encode()
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, errors.New("failed to build Luban balance request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CPA-Monitor/1.0")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Luban balance request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("Luban balance endpoint returned HTTP %d", resp.StatusCode)
	}
	var payload lubanBalancePayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return 0, errors.New("invalid Luban balance response")
	}
	if payload.Code != 0 {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = "Luban rejected the API key"
		}
		return 0, errors.New(message)
	}
	var text string
	if err := json.Unmarshal(payload.Balance, &text); err != nil {
		text = strings.TrimSpace(string(payload.Balance))
	}
	balance, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, errors.New("invalid Luban balance value")
	}
	return balance, nil
}
