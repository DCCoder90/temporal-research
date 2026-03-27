package report_test

import (
	"strings"
	"testing"
	"temporal-analyze/internal/report"
	"temporal-analyze/internal/tshark"
)

var testPackets = []tshark.Packet{
	{Src: "172.20.0.41", Dst: "172.20.0.21", Dport: "7233", Len: 512, Proto: "HTTP2"},
	{Src: "172.20.0.22", Dst: "172.20.0.10", Dport: "5432", Len: 128, Proto: "PGSQL"},
}

var testCalls = []tshark.GRPCCall{
	{T: 1.0, Src: "hello-world-starter", Dst: "temporal-frontend", Method: "StartWorkflowExecution"},
}

func TestGenerateHTML_Structure(t *testing.T) {
	trafficSeq := "sequenceDiagram\n    participant tf as temporal-frontend"
	out := report.GenerateHTML(report.HTMLInput{
		PcapName:    "test.pcap",
		Duration:    5.0,
		Packets:     testPackets,
		GRPCCalls:   testCalls,
		FlowDiagram: "flowchart LR\n    a([\"a\"])",
		SeqDiagrams: []string{"sequenceDiagram\n    participant a as a"},
		TrafficSeq:  &trafficSeq,
		FilterDesc:  "",
	})

	checks := []string{
		"<!DOCTYPE html>",
		"test.pcap",
		"Temporal Traffic Analysis",
		"mermaid",
		"flowchart LR",
		"sequenceDiagram",
		"svg-pan-zoom",
		"zoomIn",
		"Traffic Sequence Diagram",
		"gRPC Sequence Diagram",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing expected string: %q", want)
		}
	}
}

func TestGenerateHTML_NoTrafficSeq(t *testing.T) {
	out := report.GenerateHTML(report.HTMLInput{
		PcapName:    "test.pcap",
		Packets:     testPackets,
		GRPCCalls:   testCalls,
		FlowDiagram: "flowchart LR",
		SeqDiagrams: []string{"sequenceDiagram"},
		TrafficSeq:  nil,
	})
	if strings.Contains(out, "Traffic Sequence Diagram") {
		t.Error("Traffic Sequence section should be absent when TrafficSeq is nil")
	}
}

func TestGenerateHTML_FilterBadge(t *testing.T) {
	out := report.GenerateHTML(report.HTMLInput{
		PcapName:    "test.pcap",
		Packets:     testPackets,
		GRPCCalls:   nil,
		FlowDiagram: "flowchart LR",
		SeqDiagrams: []string{"sequenceDiagram"},
		FilterDesc:  "only: grpc",
	})
	if !strings.Contains(out, "only: grpc") {
		t.Error("filter description should appear in HTML output")
	}
	if !strings.Contains(out, "fff3cd") {
		t.Error("filter badge should use yellow background color")
	}
}

func TestGenerateStats_Structure(t *testing.T) {
	out := report.GenerateStats("test.pcap", testPackets, testCalls, 5.0, "")
	sections := []string{
		"# Temporal Traffic Analysis",
		"## Protocol Breakdown",
		"## Connection Matrix",
		"## gRPC Method Calls",
		"## Top Talkers",
	}
	for _, section := range sections {
		if !strings.Contains(out, section) {
			t.Errorf("stats output missing section: %q", section)
		}
	}
}

func TestGenerateStats_WithGRPCCalls(t *testing.T) {
	out := report.GenerateStats("test.pcap", testPackets, testCalls, 5.0, "only: grpc")
	if !strings.Contains(out, "StartWorkflowExecution") {
		t.Error("expected gRPC method name in stats output")
	}
	if !strings.Contains(out, "Temporal-Specific Insights") {
		t.Error("expected Temporal insights section when gRPC calls present")
	}
	if !strings.Contains(out, "only: grpc") {
		t.Error("expected filter description in stats output")
	}
}

func TestGenerateStats_NoGRPCCalls(t *testing.T) {
	out := report.GenerateStats("test.pcap", testPackets, nil, 5.0, "")
	if !strings.Contains(out, "No gRPC calls were decoded") {
		t.Error("expected fallback message when no gRPC calls")
	}
}
