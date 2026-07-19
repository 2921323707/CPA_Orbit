package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/scraper"
	"cpa-monitor/server/internal/storage"
)

type Monitor struct {
	mu             sync.RWMutex
	refreshMu      sync.Mutex
	offersPath     string
	gptPlusPath    string
	historyPath    string
	gptHistoryPath string
	alertsPath     string
	settings       *config.Store
	scraper        *scraper.Client
	gptPlusScraper *scraper.Client
	offers         model.OffersFile
	gptPlus        model.OfferFeed
	priceHistory   []model.PriceSample
	gptPlusHistory []model.PriceSample
	alerts         []model.Alert
	alertHandler   func(model.Alert)
	reset          chan struct{}
}

func (m *Monitor) SetAlertHandler(handler func(model.Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandler = handler
}

func NewMonitor(offersPath, alertsPath string, settings *config.Store, scraperClient *scraper.Client) (*Monitor, error) {
	m := &Monitor{offersPath: offersPath, gptPlusPath: filepath.Join(filepath.Dir(offersPath), "gpt_plus_offers.json"), historyPath: filepath.Join(filepath.Dir(offersPath), "offers_history.json"), gptHistoryPath: filepath.Join(filepath.Dir(offersPath), "gpt_plus_history.json"), alertsPath: alertsPath, settings: settings, scraper: scraperClient, gptPlusScraper: scraper.NewGPTPlusClient(), reset: make(chan struct{}, 1)}
	m.offers.Offers = []model.Offer{}
	m.gptPlus.Offers = []model.Offer{}
	m.gptPlus.SourceURL = scraper.GPTPlusURL
	m.priceHistory = []model.PriceSample{}
	m.gptPlusHistory = []model.PriceSample{}
	m.alerts = []model.Alert{}
	if err := storage.LoadJSON(offersPath, &m.offers); err != nil {
		return nil, fmt.Errorf("load offers: %w", err)
	}
	if m.offers.Offers == nil {
		m.offers.Offers = []model.Offer{}
	}
	if err := storage.LoadJSON(m.gptPlusPath, &m.gptPlus); err != nil {
		return nil, fmt.Errorf("load GPT Plus offers: %w", err)
	}
	if m.gptPlus.Offers == nil {
		m.gptPlus.Offers = []model.Offer{}
	}
	if m.gptPlus.SourceURL == "" {
		m.gptPlus.SourceURL = scraper.GPTPlusURL
	}
	if err := storage.LoadJSON(m.historyPath, &m.priceHistory); err != nil {
		return nil, fmt.Errorf("load offer history: %w", err)
	}
	if m.priceHistory == nil {
		m.priceHistory = []model.PriceSample{}
	}
	if err := storage.LoadJSON(m.gptHistoryPath, &m.gptPlusHistory); err != nil {
		return nil, fmt.Errorf("load GPT Plus history: %w", err)
	}
	if m.gptPlusHistory == nil {
		m.gptPlusHistory = []model.PriceSample{}
	}
	if err := storage.LoadJSON(alertsPath, &m.alerts); err != nil {
		return nil, fmt.Errorf("load alerts: %w", err)
	}
	if m.alerts == nil {
		m.alerts = []model.Alert{}
	}
	return m, nil
}

func (m *Monitor) PriceHistory() []model.PriceSample {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.PriceSample(nil), m.priceHistory...)
}

func (m *Monitor) GPTPlusPriceHistory() []model.PriceSample {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.PriceSample(nil), m.gptPlusHistory...)
}

func (m *Monitor) Offers() model.OffersFile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := m.offers
	result.Offers = append([]model.Offer(nil), m.offers.Offers...)
	return result
}

func (m *Monitor) GPTPlusOffers() model.OfferFeed {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := m.gptPlus
	result.Offers = append([]model.Offer(nil), m.gptPlus.Offers...)
	return result
}

func (m *Monitor) Alerts() []model.Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := append([]model.Alert(nil), m.alerts...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

func (m *Monitor) Refresh(ctx context.Context) (model.OffersFile, error) {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	offers, err := m.scraper.Fetch()
	if err != nil {
		return m.Offers(), err
	}
	state := model.OffersFile{Offers: offers, UpdatedAt: time.Now()}
	plusOffers, plusErr := m.gptPlusScraper.Fetch()
	plusState := model.OfferFeed{Offers: plusOffers, UpdatedAt: state.UpdatedAt, SourceURL: m.gptPlusScraper.URL}
	plusSuccess := false
	plusHistory := m.GPTPlusPriceHistory()
	if plusErr != nil {
		plusState = m.GPTPlusOffers()
		plusState.LastError = cleanRefreshError(plusErr.Error())
	} else if err := storage.SaveJSON(m.gptPlusPath, plusState); err != nil {
		plusState = m.GPTPlusOffers()
		plusState.LastError = cleanRefreshError(err.Error())
	} else {
		plusSuccess = true
		if sample, ok := averagePriceSample(state.UpdatedAt, plusOffers); ok {
			plusHistory = append(plusHistory, sample)
		}
		plusHistory = keepRecentPriceHistory(plusHistory, state.UpdatedAt)
		if err := storage.SaveJSON(m.gptHistoryPath, plusHistory); err != nil {
			plusState.LastError = cleanRefreshError(err.Error())
			plusHistory = m.GPTPlusPriceHistory()
		}
	}
	history := m.PriceHistory()
	if sample, ok := averagePriceSample(state.UpdatedAt, offers); ok {
		history = append(history, sample)
	}
	history = keepRecentPriceHistory(history, state.UpdatedAt)
	if err := storage.SaveJSON(m.offersPath, state); err != nil {
		return m.Offers(), err
	}
	if err := storage.SaveJSON(m.historyPath, history); err != nil {
		return m.Offers(), err
	}
	m.mu.Lock()
	m.offers = state
	if plusSuccess {
		m.gptPlus = plusState
	} else {
		m.gptPlus.LastError = plusState.LastError
	}
	m.priceHistory = append([]model.PriceSample(nil), history...)
	m.gptPlusHistory = append([]model.PriceSample(nil), plusHistory...)
	m.mu.Unlock()
	if len(offers) > 0 {
		m.maybeAlert(ctx, offers[0])
	}
	return state, nil
}

func cleanRefreshError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func averagePriceSample(at time.Time, offers []model.Offer) (model.PriceSample, bool) {
	var total float64
	count := 0
	for _, offer := range offers {
		if offer.Price <= 0 {
			continue
		}
		total += offer.Price
		count++
	}
	if count == 0 {
		return model.PriceSample{}, false
	}
	return model.PriceSample{At: at, Average: total / float64(count)}, true
}

func keepRecentPriceHistory(history []model.PriceSample, now time.Time) []model.PriceSample {
	cutoff := now.Add(-14 * 24 * time.Hour)
	kept := make([]model.PriceSample, 0, len(history))
	for _, sample := range history {
		if !sample.At.Before(cutoff) && !sample.At.After(now) && sample.Average > 0 {
			kept = append(kept, sample)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].At.Before(kept[j].At) })
	return kept
}

func (m *Monitor) maybeAlert(ctx context.Context, offer model.Offer) {
	settings := m.settings.Get()
	if offer.Price >= settings.Threshold {
		return
	}
	now := time.Now()
	key := offer.ItemID
	if key == "" {
		key = offer.OrderURL
	}
	m.mu.Lock()
	for _, alert := range m.alerts {
		alertKey := alert.ItemID
		if alertKey == "" {
			alertKey = alert.OrderURL
		}
		if alertKey == key && alert.Price == offer.Price && now.Sub(alert.CreatedAt) < 30*time.Minute {
			m.mu.Unlock()
			return
		}
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%.8f|%d", key, offer.Price, now.UnixNano())))
	alert := model.Alert{
		ID: hex.EncodeToString(sum[:8]), ItemID: offer.ItemID, Title: offer.Title,
		Price: offer.Price, Threshold: settings.Threshold, Merchant: offer.Merchant,
		OrderURL: offer.OrderURL, CreatedAt: now,
	}
	m.alerts = append(m.alerts, alert)
	index := len(m.alerts) - 1
	alertsCopy := append([]model.Alert(nil), m.alerts...)
	handler := m.alertHandler
	m.mu.Unlock()
	_ = storage.SaveJSON(m.alertsPath, alertsCopy)
	if handler != nil {
		handler(alert)
	}

	if settings.WebhookURL == "" {
		return
	}
	err := postWebhook(ctx, settings.WebhookURL, map[string]any{"type": "low_price", "alert": alert})
	m.mu.Lock()
	if index < len(m.alerts) && m.alerts[index].ID == alert.ID {
		if err != nil {
			m.alerts[index].WebhookError = err.Error()
		} else {
			m.alerts[index].WebhookSent = true
			m.alerts[index].WebhookSentAt = time.Now()
		}
		alertsCopy = append([]model.Alert(nil), m.alerts...)
	}
	m.mu.Unlock()
	_ = storage.SaveJSON(m.alertsPath, alertsCopy)
}

func (m *Monitor) Start(ctx context.Context) {
	go func() {
		_, _ = m.Refresh(ctx)
		timer := time.NewTimer(time.Duration(m.settings.Get().RefreshMinutes) * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.reset:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(time.Duration(m.settings.Get().RefreshMinutes) * time.Minute)
			case <-timer.C:
				_, _ = m.Refresh(ctx)
				timer.Reset(time.Duration(m.settings.Get().RefreshMinutes) * time.Minute)
			}
		}
	}()
}

func (m *Monitor) ResetSchedule() {
	select {
	case m.reset <- struct{}{}:
	default:
	}
}

func postWebhook(ctx context.Context, rawURL string, payload any) error {
	if err := config.ValidateHTTPURL(rawURL); err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return config.ValidateHTTPURL(req.URL.String())
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (m *Monitor) TestWebhook(ctx context.Context, rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("webhookUrl is not configured")
	}
	return postWebhook(ctx, rawURL, map[string]any{
		"type": "test", "message": "CPA Monitor webhook test", "sentAt": time.Now(),
	})
}
