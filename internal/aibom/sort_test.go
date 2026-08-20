package aibom

import "testing"

func newUtilizationAIBOMs() []AIBOM {
	return []AIBOM{
		{Name: "low", Data: Data{ResourceUtilization: ResourceUtilization{AvgGPUUtilizationPct: 20}}},
		{Name: "high", Data: Data{ResourceUtilization: ResourceUtilization{AvgGPUUtilizationPct: 90}}},
		{Name: "mid", Data: Data{ResourceUtilization: ResourceUtilization{AvgGPUUtilizationPct: 55}}},
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
