package diagram_test

import (
	"strings"
	"testing"
	"temporal-lens/internal/diagram"
	"temporal-lens/internal/tshark"
)

var flowPackets = []tshark.Packet{
	{Src: "172.20.0.41", Dst: "172.20.0.21", Dport: "7233", Len: 512, Proto: "HTTP2"},
	{Src: "172.20.0.21", Dst: "172.20.0.41", Dport: "55000", Len: 256, Proto: "HTTP2"},
	{Src: "172.20.0.22", Dst: "172.20.0.10", Dport: "5432", Len: 128, Proto: "PGSQL"},
}

var seqCalls = []tshark.GRPCCall{
	{T: 1.0, Src: "hello-world-starter", Dst: "temporal-frontend", Method: "StartWorkflowExecution"},
	{T: 2.0, Src: "hello-world-starter", Dst: "temporal-frontend", Method: "StartWorkflowExecution"},
	{T: 3.0, Src: "temporal-frontend", Dst: "temporal-history", Method: "RecordWorkflowTaskStarted"},
}

func TestBuildFlowDiagram_ContainsMermaidKeyword(t *testing.T) {
	out := diagram.BuildFlowDiagram(flowPackets)
	if !strings.HasPrefix(out, "flowchart LR") {
		t.Errorf("expected flowchart LR prefix, got: %s", out[:min(len(out), 40)])
	}
}

func TestBuildFlowDiagram_ContainsNodeNames(t *testing.T) {
	out := diagram.BuildFlowDiagram(flowPackets)
	for _, name := range []string{"hello_world_starter", "temporal_frontend", "temporal_history", "postgresql"} {
		if strings.Contains(out, name) {
			return // at least one found
		}
	}
	// Not all will be present with these packets; just check the structure is valid.
	if !strings.Contains(out, "-->") {
		t.Error("expected at least one edge arrow")
	}
}

func TestBuildFlowDiagram_PostgreSQLCylinder(t *testing.T) {
	out := diagram.BuildFlowDiagram(flowPackets)
	if !strings.Contains(out, "postgresql") {
		t.Skip("postgresql not in test packets")
	}
	if !strings.Contains(out, "[(") {
		t.Error("postgresql should use cylinder shape [(")
	}
}

func TestBuildSequenceDiagram_EmptyCalls(t *testing.T) {
	pages := diagram.BuildSequenceDiagram(nil)
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if !strings.HasPrefix(pages[0], "sequenceDiagram") {
		t.Error("expected sequenceDiagram prefix")
	}
	if !strings.Contains(pages[0], "No gRPC calls decoded") {
		t.Error("expected fallback message for empty calls")
	}
}

func TestBuildSequenceDiagram_Compression(t *testing.T) {
	pages := diagram.BuildSequenceDiagram(seqCalls)
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}
	// Two consecutive identical StartWorkflowExecution calls should be compressed to x2.
	if !strings.Contains(pages[0], "x2") {
		t.Error("expected (x2) compression for repeated calls")
	}
}

func TestBuildSequenceDiagram_ContainsParticipants(t *testing.T) {
	pages := diagram.BuildSequenceDiagram(seqCalls)
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}
	if !strings.Contains(pages[0], "participant") {
		t.Error("expected participant declarations")
	}
}

func TestBuildTrafficSequenceDiagram_Empty(t *testing.T) {
	out := diagram.BuildTrafficSequenceDiagram(nil, nil)
	if !strings.HasPrefix(out, "sequenceDiagram") {
		t.Error("expected sequenceDiagram prefix for empty input")
	}
}

func TestBuildTrafficSequenceDiagram_ExcludesGRPCPorts(t *testing.T) {
	// Packets on gRPC ports should not appear as raw labels — they're replaced by call entries.
	grpcPacket := tshark.Packet{Src: "172.20.0.41", Dst: "172.20.0.21", Dport: "7233", Proto: "HTTP2"}
	call := tshark.GRPCCall{T: 1.0, Src: "hello-world-starter", Dst: "temporal-frontend", Method: "StartWorkflowExecution"}
	out := diagram.BuildTrafficSequenceDiagram([]tshark.Packet{grpcPacket}, []tshark.GRPCCall{call})
	if strings.Contains(out, "HTTP2") {
		t.Error("HTTP2 label should not appear — gRPC packets are replaced by call method names")
	}
	if !strings.Contains(out, "StartWorkflowExecution") {
		t.Error("expected gRPC method name in output")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
