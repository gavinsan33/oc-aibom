// Package aibom contains types and Kubernetes API logic for the AIBOM
// custom resource (group aibom.io, kind AIBOM), produced by
// aibom-webhook-service's postprocessing job for each instrumented
// training/fine-tuning/inference workload.
package aibom

import "math"

// AIBOM mirrors the schema-enforced top-level fields of an AIBOM custom
// resource's spec, plus the free-form spec.data document (preserved
// unknown fields in the CRD, compiled by postprocess.py's compile_aibom()).
type AIBOM struct {
	Name             string
	Namespace        string
	JobName          string `json:"jobName"`
	ModelName        string `json:"modelName"`
	ExperimentIntent string `json:"experimentIntent"`
	CollectedAt      string `json:"collectedAt"`
	Data             Data   `json:"data"`
}

// Data mirrors the fields compile_aibom() writes into spec.data.
type Data struct {
	ExperimentIntent      string              `json:"experiment_intent"`
	ExperimentName        string              `json:"experiment_name"`
	ExperimentDescription string              `json:"experiment_description"`
	SourceCode            SourceCode          `json:"source_code"`
	ExecutionMetadata     ExecutionMetadata   `json:"execution_metadata"`
	Model                 Model               `json:"model"`
	Dataset               Dataset             `json:"dataset"`
	Training              *Training           `json:"training,omitempty"`
	FineTuning            *FineTuning         `json:"fine_tuning,omitempty"`
	Inference             *Inference          `json:"inference,omitempty"`
	Environment           Environment         `json:"environment"`
	ResourceUtilization   ResourceUtilization `json:"resource_utilization"`
	Metadata              Metadata            `json:"_metadata"`
}

type SourceCode struct {
	GitRepository string `json:"git_repository"`
	GitCommit     string `json:"git_commit"`
	GitBranch     string `json:"git_branch"`
	DeclaredVia   string `json:"declared_via"`
	Dirty         bool   `json:"dirty"`
}

type Pod struct {
	PodName      string `json:"pod_name"`
	PodUID       string `json:"pod_uid"`
	PodNamespace string `json:"pod_namespace"`
	PodIP        string `json:"pod_ip"`
	NodeName     string `json:"node_name"`
	StartTime    string `json:"start_time"`
}

type ExecutionMetadata struct {
	JobID     string `json:"job_id"`
	Namespace string `json:"namespace"`
	Pods      []Pod  `json:"pods"`
}

type SpeculativeDecoding struct {
	Enabled              bool   `json:"enabled"`
	DraftModel           string `json:"draft_model"`
	NumSpeculativeTokens int    `json:"num_speculative_tokens"`
}

type Model struct {
	Name                string               `json:"name"`
	Version             string               `json:"version"`
	Architecture        string               `json:"architecture"`
	Framework           string               `json:"framework"`
	Quantization        string               `json:"quantization"`
	QuantizationBits    int                  `json:"quantization_bits"`
	Dtype               string               `json:"dtype"`
	SpeculativeDecoding *SpeculativeDecoding `json:"speculative_decoding,omitempty"`
}

type DeclaredDataset struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Source      string `json:"source"`
	License     string `json:"license"`
	DeclaredVia string `json:"declared_via"`
}

type AutoDetectedDataset struct {
	DatasetName     string `json:"dataset_name"`
	Version         string `json:"version"`
	License         string `json:"license"`
	MatchesDeclared bool   `json:"matches_declared"`
	SeenVia         string `json:"seen_via"`
}

type Dataset struct {
	Declared     DeclaredDataset       `json:"declared"`
	AutoDetected []AutoDetectedDataset `json:"auto_detected"`
}

// Training's numeric fields use FlexInt/FlexFloat: postprocess.py's
// detect_trl_from_command falls back to the raw CLI string on a failed
// int()/float() conversion instead of dropping the field.
type Training struct {
	Optimizer               string    `json:"optimizer"`
	LearningRate            FlexFloat `json:"learning_rate"`
	BatchSize               FlexInt   `json:"batch_size"`
	Epochs                  FlexInt   `json:"epochs"`
	RandomSeed              FlexInt   `json:"random_seed"`
	ParallelizationStrategy string    `json:"parallelization_strategy"`
}

type FineTuning struct {
	AdaptationMethod string  `json:"adaptation_method"`
	LoRARank         FlexInt `json:"lora_rank"`
	LoRAAlpha        FlexInt `json:"lora_alpha"`
}

// Inference's numeric fields (other than MaxTokens, which is always parsed
// via _try_int from an annotation) use FlexInt/FlexFloat for the same
// reason as Training: detect_vllm_from_command falls back to the raw CLI
// string on a failed conversion.
type Inference struct {
	ServingEngine        string    `json:"serving_engine"`
	MaxModelLen          FlexInt   `json:"max_model_len"`
	TensorParallelSize   FlexInt   `json:"tensor_parallel_size"`
	PipelineParallelSize FlexInt   `json:"pipeline_parallel_size"`
	EnableExpertParallel bool      `json:"enable_expert_parallel"`
	DataParallelSize     FlexInt   `json:"data_parallel_size"`
	GPUMemoryUtilization FlexFloat `json:"gpu_memory_utilization"`
	Temperature          FlexFloat `json:"temperature"`
	TopP                 FlexFloat `json:"top_p"`
	TopK                 FlexInt   `json:"top_k"`
	MaxTokens            int       `json:"max_tokens"`
}

// GPUCount, CPUCores, and NUMANodes use FlexInt: they're read straight from
// `grep`/`nproc`-style shell command stdout in generate_snapshot.py, with no
// int() cast applied before landing in spec.data.
type Environment struct {
	GPUType          string  `json:"gpu_type"`
	GPUCount         FlexInt `json:"gpu_count"`
	CPUModel         string  `json:"cpu_model"`
	CPUCores         FlexInt `json:"cpu_cores"`
	MemoryGB         float64 `json:"memory_gb"`
	NUMANodes        FlexInt `json:"numa_nodes"`
	CUDAVersion      string  `json:"cuda_version"`
	DriverVersion    string  `json:"driver_version"`
	FrameworkVersion string  `json:"framework_version"`
	KernelVersion    string  `json:"kernel_version"`
}

type ResourceUtilization struct {
	CollectedAt              string                 `json:"collected_at"`
	AvgGPUUtilizationPct     float64                `json:"avg_gpu_utilization_pct"`
	AvgGPUMemoryUsedMiB      float64                `json:"avg_gpu_memory_used_mib"`
	AvgGPUPowerWatts         float64                `json:"avg_gpu_power_watts"`
	AvgCPUUsageCores         float64                `json:"avg_cpu_usage_cores"`
	AvgMemoryUsageGB         float64                `json:"avg_memory_usage_gb"`
	AvgNetworkReceiveMbps    float64                `json:"avg_network_receive_mbps"`
	AvgNetworkTransmitMbps   float64                `json:"avg_network_transmit_mbps"`
	GrafanaLinks             []string               `json:"grafana_links"`
	SummaryIncludesColdStart bool                   `json:"summary_includes_cold_start"`
	Note                     string                 `json:"note"`
	Metrics                  map[string]MetricStats `json:"metrics"`
}

// MetricSegments is a first/middle/last-third breakdown of one telemetry
// metric's average across a run. A pointer is used because a segment can be
// legitimately absent (a run too short to fill three slices) rather than
// zero -- see compute_metric_stats() in aibom-webhook-service's
// postprocess.py.
type MetricSegments struct {
	FirstThird  *float64 `json:"first_third"`
	MiddleThird *float64 `json:"middle_third"`
	LastThird   *float64 `json:"last_third"`
}

// segmentPctChange is (to-from)/from*100, with a from-is-zero fallback so a
// move away from (or between) all-zero segments still has a sign instead of
// dividing by zero.
func segmentPctChange(from, to float64) float64 {
	if from == 0 {
		switch {
		case to == 0:
			return 0
		case to > 0:
			return 100
		default:
			return -100
		}
	}
	return (to - from) / from * 100
}

// Trend summarizes the shape of a metric across a single run's first/middle/
// last thirds -- something a run-wide average can't show. Returns "" when
// there isn't enough segment data to say.
func (s MetricSegments) Trend() string {
	if s.FirstThird == nil || s.LastThird == nil {
		return ""
	}
	first, last := *s.FirstThird, *s.LastThird

	if s.MiddleThird != nil {
		firstToMid := segmentPctChange(first, *s.MiddleThird)
		midToLast := segmentPctChange(*s.MiddleThird, last)
		// Dipped then recovered, or spiked then dropped back: first->mid and
		// mid->last moved in opposite directions, each past the same 10%
		// threshold used for up/down below. Comparing only first vs. last
		// would call a 100->50->100 run "flat" even though it was anything
		// but steady in between.
		if firstToMid*midToLast < 0 && math.Abs(firstToMid) > 10 && math.Abs(midToLast) > 10 {
			return "volatile"
		}
	}

	switch pctChange := segmentPctChange(first, last); {
	case pctChange > 10:
		return "up"
	case pctChange < -10:
		return "down"
	default:
		return "flat"
	}
}

// slopeSymbol renders a first->second transition as "↗" (rising), "↘"
// (falling), or "→" (roughly flat), using the same 10% threshold Trend()
// uses for up/down.
func slopeSymbol(from, to float64) string {
	switch pct := segmentPctChange(from, to); {
	case pct > 10:
		return "↗"
	case pct < -10:
		return "↘"
	default:
		return "→"
	}
}

// Sparkline renders the run's first/middle/last-third shape as a compact
// two-arrow string -- e.g. "↘↗" for a dip-then-recover, "↗↗" for a steady
// climb, "→→" for flat. More visual than Trend()'s single up/down/flat/
// volatile verdict, and it needs no separate "volatile" case: a mixed shape
// just draws as a mixed arrow sequence. Returns "" unless all three thirds
// are present.
func (s MetricSegments) Sparkline() string {
	if s.FirstThird == nil || s.MiddleThird == nil || s.LastThird == nil {
		return ""
	}
	return slopeSymbol(*s.FirstThird, *s.MiddleThird) + slopeSymbol(*s.MiddleThird, *s.LastThird)
}

// MetricStats is the full min/max/avg/p95/segments breakdown of one
// telemetry metric across a run, as compiled by postprocess.py's
// compute_metric_stats(). Keyed in ResourceUtilization.Metrics by the same
// names as aibom-webhook-service's TELEMETRY_QUERIES: gpu_utilization,
// gpu_memory_used, gpu_power, cpu_usage, memory_usage, network_receive,
// network_transmit.
type MetricStats struct {
	Unit     string         `json:"unit"`
	Min      float64        `json:"min"`
	Max      float64        `json:"max"`
	Avg      float64        `json:"avg"`
	P95      float64        `json:"p95"`
	Segments MetricSegments `json:"segments"`
}

type Metadata struct {
	AIBOMVersion     string `json:"aibom_version"`
	GeneratedAt      string `json:"generated_at"`
	Generator        string `json:"generator"`
	SchemaCompliance string `json:"schema_compliance"`
	DatasetDetection string `json:"dataset_detection"`
}
