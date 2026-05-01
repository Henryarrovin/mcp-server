package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Henryarrovin/mcp-server/mcp"
)

func RegisterKubernetesTools(s *mcp.Server, namespace string) {
	run := func(args ...string) (string, error) {
		cmd := exec.Command("kubectl", args...)
		var out, errBuf bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%s: %s", err.Error(), errBuf.String())
		}
		return out.String(), nil
	}

	ns := func(args map[string]any) string {
		return mcp.GetString(args, "namespace", namespace)
	}

	// Get Pods
	s.AddTool(
		mcp.NewTool("k8s_get_pods",
			"List all pods in the namespace with their status",
			map[string]mcp.Property{
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("get", "pods", "-n", ns(args), "-o", "wide")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Describe Pod
	s.AddTool(
		mcp.NewTool("k8s_describe_pod",
			"Describe a pod — shows events, env vars, resource limits",
			map[string]mcp.Property{
				"pod_name":  mcp.Str("Full pod name"),
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			[]string{"pod_name"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("describe", "pod", mcp.GetString(args, "pod_name", ""), "-n", ns(args))
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get Logs
	s.AddTool(
		mcp.NewTool("k8s_get_logs",
			"Get logs from a deployment",
			map[string]mcp.Property{
				"deployment": mcp.Str("Deployment name e.g. auth-service payment-gateway"),
				"namespace":  mcp.Str("Kubernetes namespace default: auth"),
				"tail":       mcp.Num("Number of lines to tail default: 50"),
			},
			[]string{"deployment"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			tail := mcp.GetInt(args, "tail", 50)
			result, err := run("logs", "-n", ns(args),
				"deployment/"+mcp.GetString(args, "deployment", ""),
				fmt.Sprintf("--tail=%d", tail),
			)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get Services
	s.AddTool(
		mcp.NewTool("k8s_get_services",
			"List all services in the namespace",
			map[string]mcp.Property{
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("get", "services", "-n", ns(args))
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get Deployments
	s.AddTool(
		mcp.NewTool("k8s_get_deployments",
			"List all deployments with replica status",
			map[string]mcp.Property{
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("get", "deployments", "-n", ns(args))
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get Ingresses
	s.AddTool(
		mcp.NewTool("k8s_get_ingresses",
			"List all ingresses and their routing rules",
			map[string]mcp.Property{
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("get", "ingress", "-n", ns(args), "-o", "wide")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Restart Deployment
	s.AddTool(
		mcp.NewTool("k8s_restart_deployment",
			"Rolling restart a deployment with zero downtime",
			map[string]mcp.Property{
				"deployment": mcp.Str("Deployment name e.g. auth-service payment-gateway"),
				"namespace":  mcp.Str("Kubernetes namespace default: auth"),
			},
			[]string{"deployment"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("rollout", "restart",
				"deployment/"+mcp.GetString(args, "deployment", ""), "-n", ns(args))
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Rollout Status
	s.AddTool(
		mcp.NewTool("k8s_rollout_status",
			"Check rollout status of a deployment",
			map[string]mcp.Property{
				"deployment": mcp.Str("Deployment name"),
				"namespace":  mcp.Str("Kubernetes namespace default: auth"),
			},
			[]string{"deployment"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("rollout", "status",
				"deployment/"+mcp.GetString(args, "deployment", ""), "-n", ns(args))
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get Events
	s.AddTool(
		mcp.NewTool("k8s_get_events",
			"Get recent cluster events — useful for debugging pod failures",
			map[string]mcp.Property{
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("get", "events", "-n", ns(args), "--sort-by=.lastTimestamp")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get All
	s.AddTool(
		mcp.NewTool("k8s_get_all",
			"Get all resources in the namespace",
			map[string]mcp.Property{
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("get", "all", "-n", ns(args))
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Exec
	s.AddTool(
		mcp.NewTool("k8s_exec",
			"Execute a command inside a pod e.g. check env vars or log files",
			map[string]mcp.Property{
				"deployment": mcp.Str("Deployment name"),
				"command":    mcp.Str("Command to run e.g. env or ls /apps/logs"),
				"namespace":  mcp.Str("Kubernetes namespace default: auth"),
			},
			[]string{"deployment", "command"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			deployment := mcp.GetString(args, "deployment", "")
			command := mcp.GetString(args, "command", "")
			parts := strings.Fields(command)
			kubectlArgs := append(
				[]string{"exec", "-n", ns(args), "deployment/" + deployment, "--"},
				parts...,
			)
			result, err := run(kubectlArgs...)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Kafka Topics
	s.AddTool(
		mcp.NewTool("k8s_kafka_topics",
			"List all Kafka topics in the cluster",
			map[string]mcp.Property{
				"namespace": mcp.Str("Kubernetes namespace default: auth"),
			},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := run("exec", "-n", ns(args), "deployment/kafka", "--",
				"kafka-topics.sh", "--list", "--bootstrap-server", "localhost:9092")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Check Log Files
	s.AddTool(
		mcp.NewTool("k8s_check_logs",
			"Check Kafka-written log files inside a pod",
			map[string]mcp.Property{
				"deployment":   mcp.Str("Deployment name e.g. auth-service"),
				"service_name": mcp.Str("Log folder name e.g. auth-service payment-gateway"),
				"date":         mcp.Str("Date e.g. 2026-04-29 leave empty to list files"),
				"namespace":    mcp.Str("Kubernetes namespace default: auth"),
			},
			[]string{"deployment", "service_name"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			deployment := mcp.GetString(args, "deployment", "")
			svcName := mcp.GetString(args, "service_name", "")
			date := mcp.GetString(args, "date", "")

			var result string
			var err error

			if date == "" {
				result, err = run("exec", "-n", ns(args), "deployment/"+deployment, "--",
					"ls", "/apps/logs/"+svcName+"/")
			} else {
				result, err = run("exec", "-n", ns(args), "deployment/"+deployment, "--",
					"cat", fmt.Sprintf("/apps/logs/%s/log-%s.log", svcName, date))
			}

			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)
}
