package aibom

// CycloneDX export: an AIBOM record as a CycloneDX 1.6 document, following
// aibom-webhook-service's docs/reference/cyclonedx-mapping.md row by row.
// That page is the spec; this is its only implementation. spec.data stays
// the signed source: the document is derived from it and never adds a value
// the record doesn't have.
//
// The converter reads RawData (the generic spec.data tree), not the typed
// Data struct, for the same reason Verify does: it has to see every field,
// in whichever shape the record's era wrote it (strings before #209, no
// containers before #208).

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// uuid5(NAMESPACE_URL, "https://github.com/gavinsan33/aibom-webhook-service")
var cdxSerialNamespace = [16]byte{
	0xef, 0xb5, 0x1e, 0x52, 0xf1, 0xc6, 0x57, 0xaf,
	0xb4, 0x09, 0x62, 0x1e, 0x82, 0x6f, 0x23, 0x01,
}

var (
	cdxServingEngines  = map[string]bool{"vllm": true, "tgi": true, "sglang": true, "lmdeploy": true}
	cdxTrainingIntents = map[string]bool{"training": true, "sft": true}
	// inference.performance metrics that describe the served model, not the server.
	cdxModelQuality = map[string]bool{
		"time_to_first_token_seconds": true,
		"inter_token_latency_seconds": true,
		"generation_throughput":       true,
		"prompt_throughput":           true,
	}
	cdxStats = []string{"min", "max", "avg", "p95", "limit"}
	// Training keys whose object value is kept whole as JSON instead of flattened.
	cdxJSONLeaves = map[string]bool{"seed_history": true, "device_mesh": true}
	// ponytail: a handful of common SPDX ids (lowercased, as Hugging Face
	// writes them); anything else goes in license.name. Embed the SPDX list if
	// datasets with other licenses show up.
	cdxSPDX = func() map[string]string {
		m := map[string]string{}
		for _, id := range []string{
			"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "CC0-1.0",
			"CC-BY-4.0", "CC-BY-SA-4.0", "CC-BY-NC-4.0", "CC-BY-NC-SA-4.0",
			"GPL-3.0-only", "LGPL-3.0-only", "MPL-2.0", "ODC-By-1.0", "ODbL-1.0",
		} {
			m[strings.ToLower(id)] = id
		}
		return m
	}()
	cdxPurlName   = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)?$`)
	cdxCommit     = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	cdxDigest     = regexp.MustCompile(`^(sha256:)?[0-9a-f]{64}$`)
	cdxHasOffset  = regexp.MustCompile(`(Z|[+-]\d\d:?\d\d)$`)
	cdxLibNormRun = regexp.MustCompile(`[-_.]+`)
	cdxURL        = regexp.MustCompile(`^https?://`)
)

// Document shape. Struct field order is the output key order, so documents
// diff cleanly.

type cdxBOM struct {
	Schema       string          `json:"$schema"`
	BOMFormat    string          `json:"bomFormat"`
	SpecVersion  string          `json:"specVersion"`
	SerialNumber string          `json:"serialNumber,omitempty"`
	Version      int             `json:"version"`
	Metadata     cdxMetadata     `json:"metadata"`
	Components   []*cdxComponent `json:"components,omitempty"`
	Dependencies []cdxDependency `json:"dependencies,omitempty"`
}

type cdxMetadata struct {
	Timestamp  string         `json:"timestamp,omitempty"`
	Lifecycles []cdxLifecycle `json:"lifecycles,omitempty"`
	Tools      cdxTools       `json:"tools"`
	Component  *cdxComponent  `json:"component"`
}

type cdxLifecycle struct {
	Phase string `json:"phase"`
}

type cdxTools struct {
	Components []cdxTool `json:"components"`
}

type cdxTool struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type cdxComponent struct {
	BOMRef             string           `json:"bom-ref,omitempty"`
	Type               string           `json:"type"`
	Group              string           `json:"group,omitempty"`
	Name               string           `json:"name"`
	Data               []cdxData        `json:"data,omitempty"`
	Version            string           `json:"version,omitempty"`
	Description        string           `json:"description,omitempty"`
	Licenses           []cdxLicense     `json:"licenses,omitempty"`
	Purl               string           `json:"purl,omitempty"`
	Properties         cdxProps         `json:"properties,omitempty"`
	ModelCard          *cdxModelCard    `json:"modelCard,omitempty"`
	Pedigree           *cdxPedigree     `json:"pedigree,omitempty"`
	ExternalReferences []cdxExternalRef `json:"externalReferences,omitempty"`
}

type cdxData struct {
	Type           string       `json:"type"`
	Name           string       `json:"name"`
	Classification string       `json:"classification"`
	Contents       *cdxContents `json:"contents,omitempty"`
}

type cdxContents struct {
	URL string `json:"url"`
}

type cdxLicense struct {
	License cdxLicenseChoice `json:"license"`
}

type cdxLicenseChoice struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type cdxModelCard struct {
	ModelParameters      *cdxModelParameters      `json:"modelParameters,omitempty"`
	QuantitativeAnalysis *cdxQuantitativeAnalysis `json:"quantitativeAnalysis,omitempty"`
	Properties           cdxProps                 `json:"properties,omitempty"`
}

type cdxModelParameters struct {
	ModelArchitecture string   `json:"modelArchitecture,omitempty"`
	Datasets          []cdxRef `json:"datasets,omitempty"`
}

type cdxRef struct {
	Ref string `json:"ref"`
}

type cdxQuantitativeAnalysis struct {
	PerformanceMetrics []cdxMetric `json:"performanceMetrics"`
}

type cdxMetric struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type cdxPedigree struct {
	Ancestors []cdxComponent `json:"ancestors,omitempty"`
	Commits   []cdxCommitRef `json:"commits,omitempty"`
	Notes     string         `json:"notes,omitempty"`
}

type cdxCommitRef struct {
	UID string `json:"uid"`
}

type cdxExternalRef struct {
	Type    string    `json:"type"`
	URL     string    `json:"url"`
	Hashes  []cdxHash `json:"hashes,omitempty"`
	Comment string    `json:"comment,omitempty"`
}

type cdxHash struct {
	Alg     string `json:"alg"`
	Content string `json:"content"`
}

type cdxDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn"`
}

type cdxProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type cdxProps []cdxProperty

// add appends a property unless the value is absent (nil, "", [], {}).
// false and 0 are values and are kept.
func (p *cdxProps) add(name string, v any) {
	switch t := v.(type) {
	case nil:
		return
	case string:
		if t == "" {
			return
		}
	case []any:
		if len(t) == 0 {
			return
		}
	case map[string]any:
		if len(t) == 0 {
			return
		}
	}
	*p = append(*p, cdxProperty{name, cdxString(v)})
}

// flatten adds one property per leaf, keys camelCased, in sorted key order.
func (p *cdxProps) flatten(prefix string, obj map[string]any) {
	for _, k := range sortedKeys(obj) {
		name := prefix + ":" + camel(k)
		if m, ok := obj[k].(map[string]any); ok && !cdxJSONLeaves[k] {
			p.flatten(name, m)
		} else {
			p.add(name, obj[k])
		}
	}
}

// stats adds a telemetry block: min/max/avg/p95/limit, the per-third
// averages, then the unit, for each metric.
func (p *cdxProps) stats(prefix string, metrics map[string]any) {
	for _, metric := range sortedKeys(metrics) {
		m := obj(metrics[metric])
		for _, s := range cdxStats {
			p.add(prefix+":"+metric+":"+s, m[s])
		}
		segs := obj(m["segments"])
		for _, seg := range sortedKeys(segs) {
			p.add(prefix+":"+metric+":avg:"+seg, segs[seg])
		}
		p.add(prefix+":"+metric+":unit", m["unit"])
	}
}

// cdxString renders a record value as a property string: strings as-is,
// bools as true/false, numbers in their shortest form, lists and objects
// as compact JSON with sorted keys.
func cdxString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case float64:
		return formatNumber(t)
	case json.Number:
		return t.String()
	default:
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(v)
		return strings.TrimSuffix(b.String(), "\n")
	}
}

// formatNumber writes an integral value as an integer (the cluster hands
// back 607 as int64, a JSON file as float64; both print "607") and anything
// else in the shortest form, with an exponent only outside 1e-4..1e16.
func formatNumber(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e16 {
		return strconv.FormatInt(int64(f), 10)
	}
	if a := math.Abs(f); a != 0 && (a < 1e-4 || a >= 1e16) {
		return strconv.FormatFloat(f, 'e', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func camel(key string) string {
	parts := strings.Split(key, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int64:
		return float64(t), true
	case int:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	}
	return 0, false
}

func toInt(v any) int {
	if f, ok := num(v); ok {
		return int(f)
	}
	n, _ := strconv.Atoi(str(v))
	return n
}

// versionString renders a version value; numbers can show up in hand-set fields.
func versionString(v any) string {
	if v == nil {
		return ""
	}
	return cdxString(v)
}

// purlEscape percent-encodes a purl version the way packageurl-python
// writes it: unreserved characters, ':' and '/' kept, everything else
// (notably '+' in local version labels) encoded.
func purlEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-._~:/", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func uuid5(ns [16]byte, name string) string {
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	u := h.Sum(nil)[:16]
	u[6] = u[6]&0x0f | 0x50
	u[8] = u[8]&0x3f | 0x80
	x := hex.EncodeToString(u)
	return x[0:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:]
}

func ociPurl(repository, digest string) string {
	last := repository[strings.LastIndex(repository, "/")+1:]
	return "pkg:oci/" + last + "@" + digest + "?repository_url=" + repository
}

func modelPurl(model map[string]any) string {
	name, revision := str(model["name"]), str(model["revision"])
	if str(model["name_declared_via"]) == "oci_uri" {
		if cdxDigest.MatchString(revision) {
			if !strings.HasPrefix(revision, "sha256:") {
				revision = "sha256:" + revision
			}
			return ociPurl(name, revision)
		}
		return ""
	}
	if !cdxPurlName.MatchString(name) {
		return "" // a local path or anything else purl can't name
	}
	purl := "pkg:generic/" + name
	if cdxCommit.MatchString(revision) {
		purl += "@" + revision
	}
	if strings.Contains(name, "/") {
		purl += "?download_url=https://huggingface.co/" + name
	}
	return purl
}

func modelProps(model map[string]any) cdxProps {
	var p cdxProps
	p.add("aibomrun:model:name:declaredVia", model["name_declared_via"])
	p.add("aibomrun:model:revision", model["revision"])
	p.add("aibomrun:model:sizeBytes", model["size_bytes"])
	p.add("aibomrun:model:framework", model["framework"])
	p.add("aibomrun:model:dtype", model["dtype"])
	p.add("aibomrun:model:quantization", model["quantization"])
	p.add("aibomrun:model:quantization:bits", model["quantization_bits"])
	p.add("aibomrun:model:speculativeDecoding", model["speculative_decoding"])
	return p
}

func modelCard(data, model map[string]any, datasetRefs []string) *cdxModelCard {
	card := &cdxModelCard{}
	params := &cdxModelParameters{ModelArchitecture: str(model["architecture"])}
	for _, r := range datasetRefs {
		params.Datasets = append(params.Datasets, cdxRef{r})
	}
	if params.ModelArchitecture != "" || len(params.Datasets) > 0 {
		card.ModelParameters = params
	}

	var metrics []cdxMetric
	perfMetrics := obj(obj(obj(data["inference"])["performance"])["metrics"])
	for _, metric := range sortedKeys(perfMetrics) {
		if !cdxModelQuality[metric] {
			continue
		}
		m := obj(perfMetrics[metric])
		base := metric
		if unit := str(m["unit"]); unit != "" && !strings.HasSuffix(metric, unit) {
			base += "_" + unit
		}
		for _, s := range cdxStats[:4] {
			if m[s] != nil {
				metrics = append(metrics, cdxMetric{base + ":" + s, cdxString(m[s])})
			}
		}
		segs := obj(m["segments"])
		for _, seg := range sortedKeys(segs) {
			if segs[seg] != nil {
				metrics = append(metrics, cdxMetric{base + ":avg:" + seg, cdxString(segs[seg])})
			}
		}
	}
	if len(metrics) > 0 {
		card.QuantitativeAnalysis = &cdxQuantitativeAnalysis{metrics}
	}

	card.Properties.flatten("aibomrun:training", obj(data["training"]))
	card.Properties.flatten("aibomrun:fineTuning", obj(data["fine_tuning"]))
	if card.ModelParameters == nil && card.QuantitativeAnalysis == nil && len(card.Properties) == 0 {
		return nil
	}
	return card
}

// Discovery writes local time with no offset; postprocess reads it as UTC.
func startTimeUTC(v any) any {
	s := str(v)
	if s != "" && !cdxHasOffset.MatchString(s) {
		return s + "Z"
	}
	return v
}

// envPod is the pod whose node the environment's CPU, memory and kernel
// facts describe.
// ponytail: environment describes the first concurrent GPU pod (#115),
// which the record doesn't name; the first pod with a GPU claim is it in
// every record seen so far. Record the pod in environment if that breaks.
func envPod(pods []any) map[string]any {
	for _, p := range pods {
		pod := obj(p)
		if gpus, _ := num(pod["gpu_count"]); gpus > 0 && pod["retried_attempt"] != true {
			return pod
		}
	}
	if len(pods) > 0 {
		return obj(pods[0])
	}
	return nil
}

// splitList accepts a pre-#209 comma-separated string or a list.
func splitList(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	var out []any
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// blockDevices accepts pre-#209 lsblk lines or the [{name, size}] list.
func blockDevices(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	var out []any
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		dev := map[string]any{"name": fields[0], "size": nil}
		if len(fields) > 1 {
			dev["size"] = fields[1]
		}
		out = append(out, dev)
	}
	return out
}

func nodeProps(env map[string]any) cdxProps {
	cpu, net := obj(env["cpu"]), obj(env["network"])
	var p cdxProps
	p.add("aibomrun:node:cpu:model", env["cpu_model"])
	p.add("aibomrun:node:cpu:cores", env["cpu_cores"])
	p.add("aibomrun:node:memoryGb", env["memory_gb"])
	p.add("aibomrun:node:numaNodes", env["numa_nodes"])
	p.add("aibomrun:node:kernel:version", env["kernel_version"])
	p.add("aibomrun:node:cpu:cacheL3", cpu["cache_l3"])
	p.add("aibomrun:node:cpu:architecture", cpu["cpu_architecture"])
	p.add("aibomrun:node:cpu:coresPerSocket", cpu["cpu_cores_per_socket"])
	p.add("aibomrun:node:cpu:threadsPerCore", cpu["cpu_threads_per_core"])
	p.add("aibomrun:node:network:primaryMtu", net["primary_mtu"])
	p.add("aibomrun:node:network:rdmaDeviceCount", net["rdma_device_count"])
	p.add("aibomrun:node:network:rdmaDevices", splitList(net["rdma_devices"]))
	p.add("aibomrun:node:storage:blockDevices", blockDevices(obj(env["storage"])["block_devices"]))
	kc := obj(env["kernel_config"])
	for _, k := range sortedKeys(kc) {
		p.add("aibomrun:node:kernel:"+camel(k), kc[k])
	}
	return p
}

func hardware(env map[string]any, pods []any) []*cdxComponent {
	var out []*cdxComponent
	gpuType := str(env["gpu_type"])
	memory := list(env["gpu_memory_mb"])
	if gpuType != "" {
		for i := 0; i < toInt(env["gpu_count"]); i++ {
			var p cdxProps
			if i < len(memory) {
				p.add("aibomrun:gpu:memoryMb", memory[i])
			}
			out = append(out, &cdxComponent{BOMRef: fmt.Sprintf("gpu:%d", i), Type: "device", Name: gpuType, Properties: p})
		}
	}

	vendor := strings.ToLower(gpuType)
	driver := ""
	switch {
	case strings.Contains(vendor, "nvidia"):
		driver = "nvidia-driver"
	case strings.Contains(vendor, "amd"):
		driver = "amdgpu"
	}
	version := strings.TrimSpace(str(env["driver_version"]))
	if driver != "" && version != "" && !strings.EqualFold(version, "not available") && !strings.EqualFold(version, "n/a") {
		var p cdxProps
		if driver == "nvidia-driver" {
			p.add("aibomrun:gpu:cudaDriverVersion", env["cuda_version"])
		}
		out = append(out, &cdxComponent{BOMRef: "driver:" + driver, Type: "device-driver", Name: driver, Version: str(env["driver_version"]), Properties: p})
	}

	envNode := str(envPod(pods)["node_name"])
	seen := map[string]bool{}
	for _, p := range pods {
		node := str(obj(p)["node_name"])
		if node == "" || seen[node] {
			continue
		}
		seen[node] = true
		c := &cdxComponent{BOMRef: "node:" + node, Type: "device", Name: node}
		if node == envNode {
			c.Properties = nodeProps(env)
		}
		out = append(out, c)
	}
	return out
}

func containers(pods []any) []*cdxComponent {
	var out []*cdxComponent
	byRef := map[string]*cdxComponent{}
	names := map[string][]string{}
	for _, p := range pods {
		for _, raw := range list(obj(p)["containers"]) {
			c := obj(raw)
			image, imageID := str(c["image"]), str(c["image_id"])
			if image == "" && imageID == "" {
				continue
			}
			key := image
			if imageID != "" {
				key = imageID[strings.LastIndex(imageID, "@")+1:]
			}
			ref := "image:" + key
			if byRef[ref] == nil {
				repo, tag := image, ""
				if i := strings.Index(repo, "@"); i >= 0 {
					repo = repo[:i]
				}
				if i := strings.LastIndex(repo, ":"); i > strings.LastIndex(repo, "/") {
					repo, tag = repo[:i], repo[i+1:]
				}
				if repo == "" {
					repo = imageID
				}
				comp := &cdxComponent{BOMRef: ref, Type: "container", Name: repo, Version: tag}
				if i := strings.Index(imageID, "@sha256:"); i >= 0 {
					comp.Purl = ociPurl(imageID[:i], key)
				} else {
					comp.Properties.add("aibomrun:container:imageId", imageID)
				}
				byRef[ref] = comp
				out = append(out, comp)
			}
			if n := str(c["name"]); n != "" && !contains(names[ref], n) {
				names[ref] = append(names[ref], n)
			}
		}
	}
	for _, comp := range out {
		comp.Properties.add("aibomrun:container:names", strings.Join(names[comp.BOMRef], ","))
	}
	return out
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func software(data, env map[string]any, pods []any) []*cdxComponent {
	out := containers(pods)
	if v := str(env["python_version"]); v != "" {
		out = append(out, &cdxComponent{BOMRef: "platform:python", Type: "platform", Name: "python", Version: v, Purl: "pkg:generic/python@" + v})
	}
	libs := obj(env["libraries"])
	for _, name := range sortedKeys(libs) {
		version := versionString(libs[name])
		norm := strings.ToLower(cdxLibNormRun.ReplaceAllString(name, "-"))
		comp := &cdxComponent{BOMRef: "pypi:" + norm, Type: "library", Name: name, Version: version, Purl: "pkg:pypi/" + norm + "@" + purlEscape(version)}
		if norm == "torch" {
			comp.Properties.add("aibomrun:torch:cudaVersion", env["torch_cuda_version"])
			comp.Properties.add("aibomrun:torch:hipVersion", env["torch_hip_version"])
			comp.Properties.add("aibomrun:torch:cudnnVersion", env["cudnn_version"])
			comp.Properties.add("aibomrun:torch:ncclVersion", env["nccl_version"])
		}
		out = append(out, comp)
	}

	inference := obj(data["inference"])
	engine := str(inference["serving_engine"])
	if engine == "" {
		engine = str(obj(data["model"])["framework"])
	}
	if cdxServingEngines[engine] {
		var p cdxProps
		p.add("aibomrun:runtime:name", engine)
		for _, k := range sortedKeys(inference) {
			if k != "serving_engine" && k != "performance" {
				p.add("aibomrun:inference:"+camel(k), inference[k])
			}
		}
		perf := obj(inference["performance"])
		server := map[string]any{}
		for k, v := range obj(perf["metrics"]) {
			if !cdxModelQuality[k] {
				server[k] = v
			}
		}
		p.stats("aibomrun:inference:telemetry", server)
		if len(server) > 0 {
			p.add("aibomrun:inference:telemetry:collectedAt", perf["collected_at"])
			p.add("aibomrun:inference:telemetry:summaryIncludesColdStart", perf["summary_includes_cold_start"])
		}
		out = append(out, &cdxComponent{BOMRef: "runtime:" + engine, Type: "application", Name: engine, Version: versionString(libs[engine]), Properties: p})
	}
	return out
}

func datasets(dataset map[string]any) []*cdxComponent {
	var out []*cdxComponent
	for i, raw := range list(dataset["auto_detected"]) {
		d := obj(raw)
		name := str(d["dataset_name"])
		if name == "" {
			continue
		}
		comp := &cdxComponent{
			BOMRef:      fmt.Sprintf("dataset:%d", i),
			Type:        "data",
			Name:        name,
			Data:        []cdxData{{Type: "dataset", Name: name, Classification: "dataset"}},
			Version:     versionString(d["version"]),
			Description: str(d["description"]),
		}
		source := d["source"]
		if s := str(source); cdxURL.MatchString(s) {
			comp.Data[0].Contents = &cdxContents{s}
			source = nil
		}
		if lic := d["license"]; lic != nil && lic != "" {
			l := cdxString(lic)
			if id, ok := cdxSPDX[strings.ToLower(l)]; ok {
				comp.Licenses = []cdxLicense{{cdxLicenseChoice{ID: id}}}
			} else {
				comp.Licenses = []cdxLicense{{cdxLicenseChoice{Name: l}}}
			}
		}
		var urls []string
		if u := str(d["url"]); u != "" {
			urls = append(urls, u)
		}
		switch u := d["urls"].(type) {
		case []any:
			for _, x := range u {
				urls = append(urls, str(x))
			}
		case map[string]any:
			for _, k := range sortedKeys(u) {
				urls = append(urls, str(u[k]))
			}
		}
		for _, u := range urls {
			if u != "" {
				comp.ExternalReferences = append(comp.ExternalReferences, cdxExternalRef{Type: "distribution", URL: u})
			}
		}

		p := &comp.Properties
		p.add("aibomrun:dataset:source", source)
		if d["fingerprint"] != nil && d["fingerprint"] != "" {
			p.add("aibomrun:dataset:fingerprint", d["fingerprint"])
			p.add("aibomrun:dataset:fingerprint:kind", "metadata")
		}
		split := str(d["split"])
		if split == "" {
			var parts []string
			for _, s := range list(d["splits"]) {
				parts = append(parts, str(s))
			}
			split = strings.Join(parts, ",")
		}
		p.add("aibomrun:dataset:split", split)
		p.add("aibomrun:dataset:configName", d["config_name"])
		p.add("aibomrun:dataset:revision", d["revision"])
		p.add("aibomrun:dataset:builder", d["builder"])
		p.add("aibomrun:dataset:root", d["root"])
		p.add("aibomrun:dataset:path", d["path"])
		p.add("aibomrun:dataset:dataDir", d["data_dir"])
		p.add("aibomrun:dataset:dataFiles", d["data_files"])
		p.add("aibomrun:dataset:class", d["dataset_class"])
		p.add("aibomrun:dataset:transform", d["transform"])
		p.add("aibomrun:dataset:download", d["download"])
		p.add("aibomrun:dataset:batchSize", d["batch_size"])
		sampling := obj(d["sampling"])
		for _, k := range sortedKeys(sampling) {
			p.add("aibomrun:dataset:sampling:"+camel(k), sampling[k])
		}
		p.add("aibomrun:dataset:matchesDeclared", d["matches_declared"])
		var seen []string
		for _, s := range list(d["seen_via"]) {
			seen = append(seen, str(s))
		}
		p.add("aibomrun:dataset:seenVia", strings.Join(seen, ","))
		p.add("aibomrun:dataset:detectedVia", d["detected_via"])
		out = append(out, comp)
	}
	return out
}

func seriesReference(ref map[string]any, namespace string) *cdxExternalRef {
	var url string
	switch {
	case str(ref["kind"]) != "" && str(ref["name"]) != "":
		url = "urn:aibomrun:" + namespace + "/" + str(ref["kind"]) + "/" + str(ref["name"])
	case str(ref["configmap"]) != "":
		url = "urn:aibomrun:" + namespace + "/ConfigMap/" + str(ref["configmap"]) + "#" + str(ref["key"])
	default:
		return nil
	}
	out := &cdxExternalRef{Type: "quality-metrics", URL: url}
	if h := str(ref["sha256"]); h != "" {
		out.Hashes = []cdxHash{{"SHA-256", h}}
	}
	var comment []string
	if ref["size_bytes"] != nil {
		comment = append(comment, cdxString(ref["size_bytes"])+" bytes")
	}
	if ref["schema_version"] != nil {
		comment = append(comment, "schema "+cdxString(ref["schema_version"]))
	}
	w := obj(ref["window"])
	if w["start"] != nil && w["end"] != nil {
		s := "window " + cdxString(w["start"]) + ".." + cdxString(w["end"])
		if w["step_seconds"] != nil {
			s += " step " + cdxString(w["step_seconds"]) + "s"
		}
		comment = append(comment, s)
	}
	out.Comment = strings.Join(comment, ", ")
	return out
}

// CycloneDX returns the AIBOM as an indented CycloneDX 1.6 JSON document.
func CycloneDX(a AIBOM) ([]byte, error) {
	data := a.RawData
	em := obj(data["execution_metadata"])
	env := obj(data["environment"])
	model := obj(data["model"])
	dataset := obj(data["dataset"])
	source := obj(data["source_code"])
	pods := list(em["pods"])
	namespace := str(em["namespace"])
	if namespace == "" {
		namespace = a.Namespace
	}
	job := str(em["job_id"])
	intent := str(data["experiment_intent"])
	training := cdxTrainingIntents[intent]

	ds := datasets(dataset)
	var datasetRefs []string
	if training {
		for _, c := range ds {
			datasetRefs = append(datasetRefs, c.BOMRef)
		}
	}

	// Root: the run itself.
	var p cdxProps
	p.add("aibomrun:record:formatVersion", obj(data["_metadata"])["aibom_version"])
	p.add("aibomrun:experiment:intent", data["experiment_intent"])
	p.add("aibomrun:experiment:intent:declaredVia", data["experiment_intent_declared_via"])
	p.add("aibomrun:job:name", em["job_id"])
	p.add("aibomrun:job:triggerKind", obj(em["trigger"])["kind"])
	p.add("aibomrun:job:status", em["status"])
	p.add("aibomrun:job:durationSeconds", em["duration_seconds"])
	p.add("aibomrun:job:nodeCount", env["node_count"])
	for i, raw := range pods {
		pod := obj(raw)
		pre := fmt.Sprintf("aibomrun:pod:%d", i)
		p.add(pre+":name", pod["pod_name"])
		p.add(pre+":uid", pod["pod_uid"])
		p.add(pre+":namespace", pod["pod_namespace"])
		p.add(pre+":node", pod["node_name"])
		p.add(pre+":startTime", startTimeUTC(pod["start_time"]))
		p.add(pre+":status", pod["status"])
		p.add(pre+":exitCode", pod["exit_code"])
		p.add(pre+":restartCount", pod["restart_count"])
		p.add(pre+":lastTerminationReason", pod["last_termination_reason"])
		p.add(pre+":lastExitCode", pod["last_exit_code"])
		p.add(pre+":servingRole", pod["serving_role"])
		p.add(pre+":retriedAttempt", pod["retried_attempt"])
		p.add(pre+":gpuCount", pod["gpu_count"])
	}
	if training {
		p = append(p, modelProps(model)...)
	}
	declared := obj(dataset["declared"])
	for _, k := range []string{"name", "version", "source", "license", "config_name", "split", "declared_via"} {
		p.add("aibomrun:dataset:declared:"+camel(k), declared[k])
	}
	ru := obj(data["resource_utilization"])
	p.stats("aibomrun:telemetry", obj(ru["metrics"]))
	if len(obj(ru["metrics"])) > 0 {
		p.add("aibomrun:telemetry:collectedAt", ru["collected_at"])
		p.add("aibomrun:telemetry:summaryIncludesColdStart", ru["summary_includes_cold_start"])
	}
	p.add("aibomrun:source:gitBranch", source["git_branch"])
	p.add("aibomrun:source:declaredVia", source["declared_via"])
	p.add("aibomrun:source:dirty", source["dirty"])
	if a.Signature != "" {
		p.add("aibomrun:signature:algorithm", "Ed25519")
		p.add("aibomrun:signature:value", a.Signature)
		p.add("aibomrun:signature:publicKey", a.SignaturePublicKey)
	}

	name := str(data["experiment_name"])
	if name == "" {
		name = job
	}
	root := &cdxComponent{
		BOMRef:      "run:" + namespace + "/" + job,
		Type:        "application",
		Group:       namespace,
		Name:        name,
		Description: str(data["experiment_description"]),
		Properties:  p,
	}

	modelComp := root
	var components []*cdxComponent
	if training {
		root.Type = "machine-learning-model"
		root.Version = versionString(model["version"])
		root.Purl = modelPurl(model)
	} else if mn := str(model["name"]); mn != "" {
		modelComp = &cdxComponent{
			BOMRef:     "model:" + mn,
			Type:       "machine-learning-model",
			Name:       mn,
			Version:    versionString(model["version"]),
			Purl:       modelPurl(model),
			Properties: modelProps(model),
		}
		components = append(components, modelComp)
	}
	if training || modelComp != root {
		modelComp.ModelCard = modelCard(data, model, datasetRefs)
	}

	if base := str(model["base_model"]); base != "" {
		modelComp.Pedigree = &cdxPedigree{Ancestors: []cdxComponent{{Type: "machine-learning-model", Name: base}}}
	}
	if commit := str(source["git_commit"]); commit != "" || source["dirty"] == true {
		if root.Pedigree == nil {
			root.Pedigree = &cdxPedigree{}
		}
		if commit != "" {
			root.Pedigree.Commits = []cdxCommitRef{{commit}}
		}
		if source["dirty"] == true {
			root.Pedigree.Notes = "working tree had uncommitted changes to tracked files"
		}
	}

	if repo := str(source["git_repository"]); repo != "" {
		root.ExternalReferences = append(root.ExternalReferences, cdxExternalRef{Type: "vcs", URL: repo})
	}
	if ref := seriesReference(obj(data["telemetry_series_ref"]), namespace); ref != nil {
		root.ExternalReferences = append(root.ExternalReferences, *ref)
	}
	if a.Name != "" {
		canonical, err := canonicalizeData(data)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(canonical)
		root.ExternalReferences = append(root.ExternalReferences, cdxExternalRef{
			Type:    "attestation",
			URL:     "urn:aibomrun:" + namespace + "/AIBOM/" + a.Name,
			Hashes:  []cdxHash{{"SHA-256", hex.EncodeToString(sum[:])}},
			Comment: "the signed AIBOM record this document was derived from; Ed25519 over its RFC 8785 canonical form",
		})
	}

	components = append(components, ds...)
	components = append(components, hardware(env, pods)...)
	components = append(components, software(data, env, pods)...)

	bom := cdxBOM{
		Schema:      "http://cyclonedx.org/schema/bom-1.6.schema.json",
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.6",
		Version:     1,
		Metadata: cdxMetadata{
			Timestamp: str(obj(data["_metadata"])["generated_at"]),
			Tools:     cdxTools{[]cdxTool{{"application", "aibom-webhook", str(obj(data["_metadata"])["generator_version"])}}},
			Component: root,
		},
	}
	if uid := str(obj(em["trigger"])["uid"]); uid != "" {
		bom.SerialNumber = "urn:uuid:" + uuid5(cdxSerialNamespace, uid)
	} else if a.Name != "" {
		bom.SerialNumber = "urn:uuid:" + uuid5(cdxSerialNamespace, namespace+"/"+a.Name)
	}
	switch {
	case training:
		bom.Metadata.Lifecycles = []cdxLifecycle{{"build"}}
	case intent == "inference":
		bom.Metadata.Lifecycles = []cdxLifecycle{{"operations"}}
	}
	if len(components) > 0 {
		bom.Components = components
		dep := cdxDependency{Ref: root.BOMRef}
		for _, c := range components {
			dep.DependsOn = append(dep.DependsOn, c.BOMRef)
		}
		bom.Dependencies = []cdxDependency{dep}
	}

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(bom); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
