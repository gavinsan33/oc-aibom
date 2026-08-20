// Package aibom contains types and Kubernetes API logic for the AIBOM
// custom resource (group aibom.io, kind AIBOM), produced by
// aibom-webhook-service's postprocessing job for each instrumented
// training/fine-tuning/inference workload.
package aibom

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
	CollectedAt              string   `json:"collected_at"`
	AvgGPUUtilizationPct     float64  `json:"avg_gpu_utilization_pct"`
	AvgGPUMemoryUsedMiB      float64  `json:"avg_gpu_memory_used_mib"`
	AvgGPUPowerWatts         float64  `json:"avg_gpu_power_watts"`
	AvgCPUUsageCores         float64  `json:"avg_cpu_usage_cores"`
	AvgMemoryUsageGB         float64  `json:"avg_memory_usage_gb"`
	AvgNetworkReceiveMbps    float64  `json:"avg_network_receive_mbps"`
	AvgNetworkTransmitMbps   float64  `json:"avg_network_transmit_mbps"`
	GrafanaLinks             []string `json:"grafana_links"`
	SummaryIncludesColdStart bool     `json:"summary_includes_cold_start"`
	Note                     string   `json:"note"`
}

type Metadata struct {
	AIBOMVersion     string `json:"aibom_version"`
	GeneratedAt      string `json:"generated_at"`
	Generator        string `json:"generator"`
	SchemaCompliance string `json:"schema_compliance"`
	DatasetDetection string `json:"dataset_detection"`
}
