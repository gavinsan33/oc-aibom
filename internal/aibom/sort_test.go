package aibom

import "testing"

func newUtilizationAIBOMs() []AIBOM {
	return []AIBOM{
		{Name: "low", Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"gpu_utilization": {Avg: 20}}}}},
		{Name: "high", Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"gpu_utilization": {Avg: 90}}}}},
		{Name: "mid", Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"gpu_utilization": {Avg: 55}}}}},
	}
}

func TestSortByMetricDescending(t *testing.T) {
	items := newUtilizationAIBOMs()
	if err := SortByMetric(items, "gpu-utilization", false); err != nil {
		t.Fatalf("SortByMetric: %v", err)
	}
	if items[0].Name != "high" || items[1].Name != "mid" || items[2].Name != "low" {
		t.Fatalf("unexpected order: %v, %v, %v", items[0].Name, items[1].Name, items[2].Name)
	}
}

func TestSortByMetricAscending(t *testing.T) {
	items := newUtilizationAIBOMs()
	if err := SortByMetric(items, "gpu-utilization", true); err != nil {
		t.Fatalf("SortByMetric: %v", err)
	}
	if items[0].Name != "low" || items[1].Name != "mid" || items[2].Name != "high" {
		t.Fatalf("unexpected order: %v, %v, %v", items[0].Name, items[1].Name, items[2].Name)
	}
}

func TestSortByMetricUnknownKey(t *testing.T) {
	items := newUtilizationAIBOMs()
	if err := SortByMetric(items, "not-a-real-metric", false); err == nil {
		t.Fatal("expected error for unknown metric key, got nil")
	}
}

func newAgeAIBOMs() []AIBOM {
	return []AIBOM{
		{Name: "middle", CollectedAt: "2026-06-15T00:00:00Z"},
		{Name: "newest", CollectedAt: "2026-09-01T00:00:00Z"},
		{Name: "oldest", CollectedAt: "2026-01-01T00:00:00Z"},
	}
}

func TestSortByAgeDescending(t *testing.T) {
	items := newAgeAIBOMs()
	SortByAge(items, false)
	if items[0].Name != "oldest" || items[1].Name != "middle" || items[2].Name != "newest" {
		t.Fatalf("unexpected order: %v, %v, %v", items[0].Name, items[1].Name, items[2].Name)
	}
}

func TestSortByAgeAscending(t *testing.T) {
	items := newAgeAIBOMs()
	SortByAge(items, true)
	if items[0].Name != "newest" || items[1].Name != "middle" || items[2].Name != "oldest" {
		t.Fatalf("unexpected order: %v, %v, %v", items[0].Name, items[1].Name, items[2].Name)
	}
}

func TestSortByAgeUnparseableSortsLast(t *testing.T) {
	items := []AIBOM{
		{Name: "bad", CollectedAt: "not-a-timestamp"},
		{Name: "good", CollectedAt: "2026-01-01T00:00:00Z"},
	}
	SortByAge(items, false)
	if items[0].Name != "good" || items[1].Name != "bad" {
		t.Fatalf("expected unparseable timestamp last, got: %v, %v", items[0].Name, items[1].Name)
	}
	SortByAge(items, true)
	if items[0].Name != "good" || items[1].Name != "bad" {
		t.Fatalf("expected unparseable timestamp last, got: %v, %v", items[0].Name, items[1].Name)
	}
}
