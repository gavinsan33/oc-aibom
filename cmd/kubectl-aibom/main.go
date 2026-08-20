// Command kubectl-aibom is a kubectl/oc plugin (invoked as `kubectl aibom`
// or `oc aibom`) that makes it easier to list, inspect, and compare AIBOM
// custom resources (group aibom.io) than raw `oc get aibom -o yaml`.
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func main() {
	configFlags := genericclioptions.NewConfigFlags(true)

	root := &cobra.Command{
		Use:   "aibom",
		Short: "List, inspect, and compare AIBOM custom resources",
	}
	configFlags.AddFlags(root.PersistentFlags())

	var allNamespaces bool
	var modelFilter, intentFilter, quantFilter string
	var driftOnly bool
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "get"},
		Short:   "List AIBOMs, optionally filtered by model/intent/quantization",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			ns := namespace
			if allNamespaces {
				ns = ""
			}
			items, err := aibom.List(context.Background(), client, ns)
			if err != nil {
				return err
			}
			items = aibom.Apply(items, aibom.Filter{
				Model:        modelFilter,
				Intent:       intentFilter,
				Quantization: quantFilter,
			})
			if driftOnly {
				items = aibom.DriftOnly(items)
			}
			printList(items, allNamespaces)
			return nil
		},
	}
	listCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "list AIBOMs across all namespaces")
	listCmd.Flags().StringVar(&modelFilter, "model", "", "filter by model.name")
	listCmd.Flags().StringVar(&intentFilter, "intent", "", "filter by experiment intent (training|sft|inference)")
	listCmd.Flags().StringVar(&quantFilter, "quantization", "", "filter by model.quantization")
	listCmd.Flags().BoolVar(&driftOnly, "drift-only", false, "only show AIBOMs where auto-detected dataset(s) disagree with the declared dataset")

	getCmd := &cobra.Command{
		Use:   "describe <name>",
		Short: "Print a human-readable summary of a single AIBOM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			a, err := aibom.Get(context.Background(), client, namespace, args[0])
			if err != nil {
				return err
			}
			printDescribe(a)
			return nil
		},
	}

	diffCmd := &cobra.Command{
		Use:   "diff <name-a> <name-b>",
		Short: "Show field-level differences between two AIBOMs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			ctx := context.Background()
			a, err := aibom.Get(ctx, client, namespace, args[0])
			if err != nil {
				return err
			}
			b, err := aibom.Get(ctx, client, namespace, args[1])
			if err != nil {
				return err
			}
			printDiff(args[0], args[1], aibom.Diff(a, b))
			return nil
		},
	}

	root.AddCommand(listCmd, getCmd, diffCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildClient(configFlags *genericclioptions.ConfigFlags) (dynamic.Interface, string, error) {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, "", fmt.Errorf("building kube config: %w", err)
	}
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, "", fmt.Errorf("building dynamic client: %w", err)
	}
	namespace, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("resolving namespace: %w", err)
	}
	return client, namespace, nil
}

func printList(items []aibom.AIBOM, allNamespaces bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if allNamespaces {
		fmt.Fprintln(w, "NAMESPACE\tNAME\tJOB\tMODEL\tINTENT\tQUANTIZATION\tGPU TYPE\tCOLLECTED AT")
		for _, a := range items {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				a.Namespace, a.Name, a.JobName, a.Data.Model.Name, a.ExperimentIntent,
				a.Data.Model.Quantization, a.Data.Environment.GPUType, a.CollectedAt)
		}
	} else {
		fmt.Fprintln(w, "NAME\tJOB\tMODEL\tINTENT\tQUANTIZATION\tGPU TYPE\tCOLLECTED AT")
		for _, a := range items {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				a.Name, a.JobName, a.Data.Model.Name, a.ExperimentIntent,
				a.Data.Model.Quantization, a.Data.Environment.GPUType, a.CollectedAt)
		}
	}
	w.Flush()
	if len(items) == 0 {
		fmt.Println("No AIBOMs found.")
	}
}

func printDescribe(a aibom.AIBOM) {
	fmt.Printf("Name:              %s\n", a.Name)
	fmt.Printf("Namespace:         %s\n", a.Namespace)
	fmt.Printf("Job:               %s\n", a.JobName)
	fmt.Printf("Experiment Intent: %s\n", a.ExperimentIntent)
	fmt.Printf("Collected At:      %s\n", a.CollectedAt)
	fmt.Println()
	fmt.Println("Model:")
	fmt.Printf("  Name:          %s\n", a.Data.Model.Name)
	fmt.Printf("  Version:       %s\n", a.Data.Model.Version)
	fmt.Printf("  Architecture:  %s\n", a.Data.Model.Architecture)
	fmt.Printf("  Framework:     %s\n", a.Data.Model.Framework)
	fmt.Printf("  Quantization:  %s (%d-bit)\n", a.Data.Model.Quantization, a.Data.Model.QuantizationBits)
	fmt.Println()
	fmt.Println("Dataset:")
	fmt.Printf("  Declared:      %s %s (license: %s)\n", a.Data.Dataset.Declared.Name, a.Data.Dataset.Declared.Version, a.Data.Dataset.Declared.License)
	for _, d := range a.Data.Dataset.AutoDetected {
		match := "matches declared"
		if !d.MatchesDeclared {
			match = "DOES NOT MATCH DECLARED"
		}
		fmt.Printf("  Auto-detected: %s %s (license: %s) — %s\n", d.DatasetName, d.Version, d.License, match)
	}
	fmt.Println()
	fmt.Println("Source:")
	fmt.Printf("  Repository:    %s\n", a.Data.SourceCode.GitRepository)
	fmt.Printf("  Commit:        %s (branch: %s, dirty: %v)\n", a.Data.SourceCode.GitCommit, a.Data.SourceCode.GitBranch, a.Data.SourceCode.Dirty)
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Printf("  GPU:           %s x%d\n", a.Data.Environment.GPUType, a.Data.Environment.GPUCount)
	fmt.Printf("  CUDA/Driver:   %s / %s\n", a.Data.Environment.CUDAVersion, a.Data.Environment.DriverVersion)
	fmt.Printf("  Framework:     %s\n", a.Data.Environment.FrameworkVersion)
}

func printDiff(nameA, nameB string, diffs []aibom.FieldDiff) {
	if len(diffs) == 0 {
		fmt.Printf("No differences found between %s and %s (across compared fields).\n", nameA, nameB)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(w, "FIELD\t%s\t%s\n", nameA, nameB)
	for _, d := range diffs {
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.Field, d.A, d.B)
	}
	w.Flush()
}
