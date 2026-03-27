package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"temporal-analyze/internal/diagram"
	"temporal-analyze/internal/filter"
	"temporal-analyze/internal/report"
	"temporal-analyze/internal/tshark"

	"golang.org/x/sync/errgroup"
)

// ErrNoPacketsAfterFilter is returned when filtering removes all packets.
var ErrNoPacketsAfterFilter = errors.New("no packets remain after filtering")

// Options controls how the analysis is run.
type Options struct {
	Filter filter.FilterOptions
}

// Result holds the complete analysis output.
type Result struct {
	PcapName      string
	Duration      float64
	TotalBytes    int
	PacketCount   int
	GRPCCount     int
	FilterDesc    string
	FlowDiagram   string
	SeqDiagram    string
	TrafficSeq    *string // nil when suppressed (grpc-only filter)
	StatsMarkdown string  // same content written to _stats.md by the CLI
	// Full slices retained for export and querying.
	Packets   []tshark.Packet
	GRPCCalls []tshark.GRPCCall
}

// Run is the single entry point for both CLI and GUI.
// It extracts packets and gRPC calls concurrently, applies filters, builds diagrams,
// and returns a Result ready for display or export.
func Run(pcapPath string, opts Options) (*Result, error) {
	if _, err := os.Stat(pcapPath); err != nil {
		return nil, fmt.Errorf("pcap file not found: %w", err)
	}

	// Extract packets and gRPC calls concurrently.
	var packets []tshark.Packet
	var grpcCalls []tshark.GRPCCall
	var duration float64

	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		var err error
		packets, duration, err = tshark.ExtractPackets(pcapPath)
		return err
	})
	g.Go(func() error {
		var err error
		grpcCalls, err = tshark.ExtractGRPCCalls(pcapPath)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("extracting capture data: %w", err)
	}

	if len(packets) == 0 {
		return nil, fmt.Errorf("no IP packets found in capture")
	}

	// Apply filters.
	packets, grpcCalls = filter.Apply(packets, grpcCalls, opts.Filter)
	if len(packets) == 0 {
		return nil, ErrNoPacketsAfterFilter
	}

	filterDesc := filter.Describe(opts.Filter)

	totalBytes := 0
	for _, p := range packets {
		totalBytes += p.Len
	}

	// Build diagrams.
	flowDiagram := diagram.BuildFlowDiagram(packets)
	seqDiagram := diagram.BuildSequenceDiagram(grpcCalls)

	var trafficSeq *string
	if filter.ShowTrafficSeq(opts.Filter) {
		ts := diagram.BuildTrafficSequenceDiagram(packets, grpcCalls)
		trafficSeq = &ts
	}

	statsMarkdown := report.GenerateStats(filepath.Base(pcapPath), packets, grpcCalls, duration, filterDesc)

	return &Result{
		PcapName:      filepath.Base(pcapPath),
		Duration:      duration,
		TotalBytes:    totalBytes,
		PacketCount:   len(packets),
		GRPCCount:     len(grpcCalls),
		FilterDesc:    filterDesc,
		FlowDiagram:   flowDiagram,
		SeqDiagram:    seqDiagram,
		TrafficSeq:    trafficSeq,
		StatsMarkdown: statsMarkdown,
		Packets:       packets,
		GRPCCalls:     grpcCalls,
	}, nil
}

// Export writes _flow.html and _stats.md to the same directory as the pcap file.
// Returns the absolute paths of the two written files.
func Export(pcapPath string, opts Options) ([]string, error) {
	result, err := Run(pcapPath, opts)
	if err != nil {
		return nil, err
	}
	return WriteResult(pcapPath, result)
}

// WriteResult writes the HTML and Markdown files for a pre-computed Result.
// Returns the absolute paths of the two written files.
func WriteResult(pcapPath string, result *Result) ([]string, error) {
	dir := filepath.Dir(pcapPath)
	stem := stemOf(pcapPath)

	htmlPath := filepath.Join(dir, stem+"_flow.html")
	statsPath := filepath.Join(dir, stem+"_stats.md")

	htmlContent := report.GenerateHTML(report.HTMLInput{
		PcapName:    result.PcapName,
		Duration:    result.Duration,
		Packets:     result.Packets,
		GRPCCalls:   result.GRPCCalls,
		FlowDiagram: result.FlowDiagram,
		SeqDiagram:  result.SeqDiagram,
		TrafficSeq:  result.TrafficSeq,
		FilterDesc:  result.FilterDesc,
	})
	statsContent := result.StatsMarkdown

	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		return nil, fmt.Errorf("writing HTML: %w", err)
	}
	if err := os.WriteFile(statsPath, []byte(statsContent), 0644); err != nil {
		return nil, fmt.Errorf("writing stats: %w", err)
	}

	absHTML, _ := filepath.Abs(htmlPath)
	absStats, _ := filepath.Abs(statsPath)
	return []string{absHTML, absStats}, nil
}

func stemOf(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return base[:len(base)-len(ext)]
}
