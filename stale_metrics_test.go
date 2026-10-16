package main

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func isolatedMetrics(t *testing.T, ttl time.Duration) *expiringMetrics {
	t.Helper()
	e := newExpiringMetrics(ttl)
	for _, v := range e.vectors {
		v.Reset()
	}
	t.Cleanup(func() {
		for _, v := range e.vectors {
			v.Reset()
		}
	})
	return e
}

func scraper(t *testing.T, e *expiringMetrics) func() string {
	t.Helper()
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(e)
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return func() string {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
		if response.Code != 200 {
			t.Fatalf("scrape failed: %s", response.Body.String())
		}
		return response.Body.String()
	}
}

func TestStaleSeriesExpireAndReturn(t *testing.T) {
	e := isolatedMetrics(t, 10*time.Minute)
	now := time.Unix(1000, 0)
	e.now = func() time.Time { return now }
	scrape := scraper(t, e)
	c, f, h, b, p := 20.0, 68.0, int32(50), 1, int32(42)
	// A non-power channel exercises removal of derived channels 0, 1 and 2.
	msg := Message{Model: "sensor", Time: "2099-01-01", TemperatureC: &c, TemperatureF: &f,
		Humidity: &h, BatteryOK: &b, Power0W: &p, Power1W: &p, Power2W: &p}
	e.observe(msg, "old", "A", "Zenna's Room")
	if len(e.lastSeen) != 9 {
		t.Fatalf("tracked %d series, want 9", len(e.lastSeen))
	}
	now = now.Add(9 * time.Minute)
	e.observe(msg, "new", "A", "Zenna's Room")
	if got := scrape(); !strings.Contains(got, `id="old"`) || !strings.Contains(got, `id="new"`) {
		t.Fatalf("series expired before timeout: %s", got)
	}
	// At the exact deadline, scraping alone removes all old metric families.
	now = now.Add(time.Minute)
	got := scrape()
	if strings.Contains(got, `id="old"`) || !strings.Contains(got, `id="new"`) {
		t.Fatalf("expiry did not preserve only the active ID: %s", got)
	}
	if len(e.lastSeen) != 9 {
		t.Fatalf("expired tracking entries were retained: %d", len(e.lastSeen))
	}
	now = now.Add(9 * time.Minute)
	if got := scrape(); got != "" {
		t.Fatalf("silent input retained metrics: %s", got)
	}
	if len(e.lastSeen) != 0 {
		t.Fatal("tracking entries leaked")
	}
	e.observe(msg, "old", "A", "Zenna's Room")
	got = scrape()
	if !strings.Contains(got, `rtl_433_packets_received{channel="A",id="old",location="Zenna's Room",model="sensor"} 1`) {
		t.Fatalf("returning sensor did not recreate its counter: %s", got)
	}
}

func TestPartialMessagesDoNotKeepOldFieldsAlive(t *testing.T) {
	e := isolatedMetrics(t, time.Minute)
	now := time.Unix(1000, 0)
	e.now = func() time.Time { return now }
	scrape := scraper(t, e)
	c, h := 20.0, int32(50)
	e.observe(Message{Model: "sensor", TemperatureC: &c, Humidity: &h}, "1", "1", "room")
	now = now.Add(30 * time.Second)
	e.observe(Message{Model: "sensor", Humidity: &h}, "1", "1", "room")
	now = now.Add(30 * time.Second)
	got := scrape()
	if strings.Contains(got, "rtl_433_temperature_celsius") || !strings.Contains(got, "rtl_433_humidity{") || !strings.Contains(got, "rtl_433_timestamp_seconds{") {
		t.Fatalf("partial packet refreshed an absent field or lost fresh ones: %s", got)
	}
}

func TestExpiryDisabled(t *testing.T) {
	e := isolatedMetrics(t, 0)
	now := time.Unix(1000, 0)
	e.now = func() time.Time { return now }
	scrape := scraper(t, e)
	e.observe(Message{Model: "sensor"}, "1", "", "")
	before := scrape()
	now = now.Add(365 * 24 * time.Hour)
	if got := scrape(); got != before || got == "" {
		t.Fatalf("disabled expiry removed metrics: %s", got)
	}
	if len(e.lastSeen) != 0 {
		t.Fatal("disabled expiry should not accumulate tracking entries")
	}
}

func TestConcurrentUpdatesAndScrapes(t *testing.T) {
	e := isolatedMetrics(t, time.Nanosecond)
	scrape := scraper(t, e)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			for j := 0; j < 50; j++ {
				if i%2 == 0 {
					scrape()
				} else {
					e.observe(Message{Model: "sensor"}, fmt.Sprint(i), "1", "room")
				}
			}
		}(i)
	}
	workers.Wait()
}
