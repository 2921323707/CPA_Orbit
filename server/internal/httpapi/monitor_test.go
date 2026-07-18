package httpapi

import (
	"testing"
	"time"

	"cpa-monitor/server/internal/model"
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
