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

type Training struct {
	Optimizer               string  `json:"optimizer"`
	LearningRate            float64 `json:"learning_rate"`
	BatchSize               int     `json:"batch_size"`
	Epochs                  int     `json:"epochs"`
	RandomSeed              int     `json:"random_seed"`
	ParallelizationStrategy string  `json:"parallelization_strategy"`
}

type FineTuning struct {
	AdaptationMethod string `json:"adaptation_method"`
	LoRARank         int    `json:"lora_rank"`
	LoRAAlpha        int    `json:"lora_alpha"`
}

type Inference struct {
	ServingEngine        string  `json:"serving_engine"`
	MaxModelLen          int     `json:"max_model_len"`
	TensorParallelSize   int     `json:"tensor_parallel_size"`
	PipelineParallelSize int     `json:"pipeline_parallel_size"`
	EnableExpertParallel bool    `json:"enable_expert_parallel"`
	DataParallelSize     int     `json:"data_parallel_size"`
	GPUMemoryUtilization float64 `json:"gpu_memory_utilization"`
	Temperature          float64 `json:"temperature"`
	TopP                 float64 `json:"top_p"`
	TopK                 int     `json:"top_k"`
	MaxTokens            int     `json:"max_tokens"`
}

type Environment struct {
	GPUType          string  `json:"gpu_type"`
	GPUCount         int     `json:"gpu_count"`
	CPUModel         string  `json:"cpu_model"`
	CPUCores         int     `json:"cpu_cores"`
	MemoryGB         float64 `json:"memory_gb"`
	NUMANodes        int     `json:"numa_nodes"`
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
