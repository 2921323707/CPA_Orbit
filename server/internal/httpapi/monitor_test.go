package httpapi

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/storage"
)

func TestAveragePriceSampleUsesAllPositiveOffers(t *testing.T) {
	at := time.Date(2026, 7, 18, 12, 0, 0, 0, time.Local)
	sample, ok := averagePriceSample(at, []model.Offer{{Price: 0.66}, {Price: 0.31}, {Price: 0.92}, {Price: 0}})
	if !ok || !sample.At.Equal(at) || sample.Average != 0.63 {
		t.Fatalf("unexpected price sample: %+v", sample)
	}
}

func TestAveragePriceSampleRejectsEmptyPrices(t *testing.T) {
	if _, ok := averagePriceSample(time.Now(), []model.Offer{{Price: 0}}); ok {
		t.Fatal("expected sample without a valid price to be rejected")
	}
}

func TestKeepRecentPriceHistoryRetainsFourteenDays(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	history := []model.PriceSample{
		{At: now.Add(-15 * 24 * time.Hour), Average: 0.4},
		{At: now.Add(-2 * time.Hour), Average: 0.6},
		{At: now.Add(-24 * time.Hour), Average: 0.5},
	}
	kept := keepRecentPriceHistory(history, now)
	if len(kept) != 2 || kept[0].Average != 0.5 || kept[1].Average != 0.6 {
		t.Fatalf("unexpected retained history: %+v", kept)
	}
}

func TestMaybeAlertSeparatesK12AndGPTPlusOffers(t *testing.T) {
	root := t.TempDir()
	settings, err := config.NewStore(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	monitor := &Monitor{
		settings:   settings,
		alertsPath: filepath.Join(root, "alerts.json"),
		alerts:     []model.Alert{},
	}
	offer := model.Offer{ItemID: "shared-id", Title: "测试报价", Price: 0.5}

	monitor.maybeAlert(context.Background(), "k12", offer)
	monitor.maybeAlert(context.Background(), "gpt-plus", offer)

	alerts := monitor.Alerts()
	if len(alerts) != 2 {
		t.Fatalf("got %d alerts, want separate K12 and GPT Plus alerts", len(alerts))
	}
	sources := map[string]bool{}
	for _, alert := range alerts {
		sources[alert.Source] = true
	}
	if !sources["k12"] || !sources["gpt-plus"] {
		t.Fatalf("unexpected alert sources: %+v", sources)
	}
}

func TestDeletePriceSamplePersistsSelectedHistory(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 7, 20, 12, 30, 0, 0, time.UTC)
	historyPath := filepath.Join(root, "offers_history.json")
	monitor := &Monitor{
		historyPath: historyPath,
		priceHistory: []model.PriceSample{
			{At: at.Add(-time.Hour), Average: 0.5},
			{At: at, Average: 180},
		},
		gptHistoryPath: filepath.Join(root, "gpt_plus_history.json"),
		gptPlusHistory: []model.PriceSample{},
	}

	deleted, err := monitor.DeletePriceSample("k12", at)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted || len(monitor.PriceHistory()) != 1 || monitor.PriceHistory()[0].Average != 0.5 {
		t.Fatalf("unexpected retained history: %+v", monitor.PriceHistory())
	}
	var persisted []model.PriceSample
	if err := storage.LoadJSON(historyPath, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].Average != 0.5 {
		t.Fatalf("unexpected persisted history: %+v", persisted)
	}
}

func TestDeletePriceSampleAcceptsJavaScriptMillisecondPrecision(t *testing.T) {
	root := t.TempDir()
	storedAt := time.Date(2026, 7, 20, 12, 30, 0, 925592100, time.FixedZone("CST", 8*60*60))
	requestedAt := storedAt.Truncate(time.Millisecond)
	monitor := &Monitor{
		historyPath: filepath.Join(root, "offers_history.json"),
		priceHistory: []model.PriceSample{
			{At: storedAt, Average: 188},
		},
	}

	deleted, err := monitor.DeletePriceSample("k12", requestedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted || len(monitor.PriceHistory()) != 0 {
		t.Fatalf("expected millisecond-precision request to delete stored sample, deleted=%v history=%+v", deleted, monitor.PriceHistory())
	}
}

func TestKeepRecentAlertsRetainsNewestTen(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	alerts := make([]model.Alert, 0, 12)
	for i := 0; i < 12; i++ {
		alerts = append(alerts, model.Alert{ID: fmt.Sprintf("alert-%02d", i), CreatedAt: base.Add(time.Duration(i) * time.Minute)})
	}

	kept := keepRecentAlerts(alerts, 10)
	if len(kept) != 10 || kept[0].ID != "alert-02" || kept[9].ID != "alert-11" {
		t.Fatalf("unexpected retained alerts: %+v", kept)
	}
}

func TestMaybeAlertPersistsAtMostTenRecords(t *testing.T) {
	root := t.TempDir()
	settings, err := config.NewStore(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	alertsPath := filepath.Join(root, "alerts.json")
	monitor := &Monitor{settings: settings, alertsPath: alertsPath, alerts: []model.Alert{}}
	for i := 0; i < 12; i++ {
		monitor.maybeAlert(context.Background(), "k12", model.Offer{
			ItemID: fmt.Sprintf("item-%02d", i), Title: "测试报价", Price: 0.5,
		})
	}

	if got := len(monitor.Alerts()); got != maxRetainedAlerts {
		t.Fatalf("got %d in-memory alerts, want %d", got, maxRetainedAlerts)
	}
	var persisted []model.Alert
	if err := storage.LoadJSON(alertsPath, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != maxRetainedAlerts {
		t.Fatalf("got %d persisted alerts, want %d", len(persisted), maxRetainedAlerts)
	}
}
