//go:build !nogui

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"temporal-analyze/internal/analysis"
	"temporal-analyze/internal/config"
	"temporal-analyze/internal/filter"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App holds the Wails application state.
type App struct {
	ctx          context.Context
	db           *sql.DB          // in-memory SQLite DB populated after each Analyze call
	lastResult   *analysis.Result // cached from last Analyze call; used by Export
	lastPcapPath string           // pcap path that produced lastResult
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// AnalysisOptions is the JS-friendly options struct passed from the frontend.
type AnalysisOptions struct {
	Only           []string `json:"Only"`
	Exclude        []string `json:"Exclude"`
	OnlyHosts      []string `json:"OnlyHosts"`
	ExcludeHosts   []string `json:"ExcludeHosts"`
	NoInterservice bool     `json:"NoInterservice"`
}

// AnalysisResult is the JS-friendly result returned to the frontend.
type AnalysisResult struct {
	PcapName      string  `json:"PcapName"`
	Duration      float64 `json:"Duration"`
	TotalBytes    int     `json:"TotalBytes"`
	PacketCount   int     `json:"PacketCount"`
	GRPCCount     int     `json:"GRPCCount"`
	FilterDesc    string  `json:"FilterDesc"`
	FlowDiagram   string  `json:"FlowDiagram"`
	SeqDiagrams   []string `json:"SeqDiagrams"`
	TrafficSeq    string  `json:"TrafficSeq"`    // empty string = suppressed
	StatsMarkdown string  `json:"StatsMarkdown"` // same content as _stats.md
}

// Analyze runs the full analysis pipeline and returns the result.
// Called from the frontend via Wails JS bindings.
func (a *App) Analyze(pcapPath string, opts AnalysisOptions) (*AnalysisResult, error) {
	result, err := analysis.Run(pcapPath, analysis.Options{
		Filter: filter.FilterOptions{
			Only:           opts.Only,
			Exclude:        opts.Exclude,
			OnlyHosts:      opts.OnlyHosts,
			ExcludeHosts:   opts.ExcludeHosts,
			NoInterservice: opts.NoInterservice,
		},
	})
	if err != nil {
		if errors.Is(err, analysis.ErrNoPacketsAfterFilter) {
			return nil, fmt.Errorf("no packets matched your filter — try relaxing it")
		}
		return nil, err
	}

	// Cache result for Export to reuse without re-running tshark.
	a.lastResult = result
	a.lastPcapPath = pcapPath

	// Rebuild the in-memory query DB for the new analysis result.
	if a.db != nil {
		a.db.Close()
	}
	a.db, err = populateDB(result)
	if err != nil {
		return nil, fmt.Errorf("building query DB: %w", err)
	}

	trafficSeq := ""
	if result.TrafficSeq != nil {
		trafficSeq = *result.TrafficSeq
	}

	return &AnalysisResult{
		PcapName:      result.PcapName,
		Duration:      result.Duration,
		TotalBytes:    result.TotalBytes,
		PacketCount:   result.PacketCount,
		GRPCCount:     result.GRPCCount,
		FilterDesc:    result.FilterDesc,
		FlowDiagram:   result.FlowDiagram,
		SeqDiagrams:   result.SeqDiagrams,
		TrafficSeq:    trafficSeq,
		StatsMarkdown: result.StatsMarkdown,
	}, nil
}

// Export writes the HTML and Markdown report files adjacent to the pcap.
// Reuses the cached analysis result from the last Analyze call if the pcap
// path matches, avoiding a second tshark pass. Falls back to re-running
// analysis if the paths differ.
func (a *App) Export(pcapPath string, opts AnalysisOptions) ([]string, error) {
	if a.lastResult != nil && a.lastPcapPath == pcapPath {
		return analysis.WriteResult(pcapPath, a.lastResult)
	}
	return analysis.Export(pcapPath, analysis.Options{
		Filter: filter.FilterOptions{
			Only:           opts.Only,
			Exclude:        opts.Exclude,
			OnlyHosts:      opts.OnlyHosts,
			ExcludeHosts:   opts.ExcludeHosts,
			NoInterservice: opts.NoInterservice,
		},
	})
}

// OpenFileDialog opens a native file picker and returns the selected path.
func (a *App) OpenFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select PCAP file",
		Filters: []runtime.FileFilter{
			{DisplayName: "PCAP files (*.pcap)", Pattern: "*.pcap"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

// ResolveIP maps a container IP to its name.
func (a *App) ResolveIP(ip string) string {
	return config.Resolve(ip)
}
