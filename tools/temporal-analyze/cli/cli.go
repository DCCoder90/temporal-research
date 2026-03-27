package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"temporal-analyze/internal/analysis"
	"temporal-analyze/internal/config"
	"temporal-analyze/internal/filter"
	"temporal-analyze/internal/tshark"

	"github.com/spf13/cobra"
)

const version = "1.0.0"

// Run is the CLI entry point. Called from both main.go (!nogui) and main_nogui.go (nogui).
func Run(args []string) {
	cmd := buildCommand()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		// cobra already printed the error
		os.Exit(1)
	}
}

func buildCommand() *cobra.Command {
	var only, exclude, onlyHost, excludeHost string
	var noInterservice, quiet, jsonOut bool

	cmd := &cobra.Command{
		Use:     "temporal-analyze <pcap-file>",
		Short:   "Analyze a Temporal .pcap capture and produce flow diagrams and statistics.",
		Long:    longDesc,
		Version: version,
		Args:    cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := buildOptions(only, exclude, onlyHost, excludeHost, noInterservice)

			progress := func(format string, a ...any) {
				if !quiet {
					fmt.Fprintf(os.Stderr, format, a...)
				}
			}

			filterDesc := filter.Describe(opts.Filter)
			progress("Analyzing %s ...\n", args[0])
			if filterDesc != "" {
				progress("  Filter: %s\n", filterDesc)
			}

			progress("  [1/2] Extracting packet data and decoding gRPC calls ...\n")
			result, err := analysis.Run(args[0], opts)
			if err != nil {
				if errors.Is(err, analysis.ErrNoPacketsAfterFilter) {
					return fmt.Errorf("no packets remain after filtering — try relaxing the filter")
				}
				return err
			}

			progress("        %s packets  |  %s  |  %.1fs window  |  %s gRPC calls\n",
				config.FmtNum(result.PacketCount),
				config.FmtBytes(result.TotalBytes),
				result.Duration,
				config.FmtNum(result.GRPCCount),
			)

			if jsonOut {
				return writeJSON(result)
			}

			progress("  [2/2] Writing output files ...\n")
			paths, err := analysis.WriteResult(args[0], result)
			if err != nil {
				return err
			}

			progress("\n✓  Done.\n")
			progress("   Flow diagram : %s\n", paths[0])
			progress("   Statistics   : %s\n", paths[1])
			progress("\n")
			return nil
		},
	}

	cmd.Flags().StringVarP(&only, "only", "o", "", "comma-separated protocols to include (all others hidden)")
	cmd.Flags().StringVarP(&exclude, "exclude", "x", "", "comma-separated protocols to exclude")
	cmd.Flags().StringVar(&onlyHost, "only-host", "", "comma-separated hosts (names or IPs) to include")
	cmd.Flags().StringVar(&excludeHost, "exclude-host", "", "comma-separated hosts to exclude")
	cmd.Flags().BoolVar(&noInterservice, "no-interservice", false,
		"exclude temporal-history, temporal-matching, temporal-internal-worker")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress progress output")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "write JSON analysis result to stdout instead of HTML/Markdown files")

	// Short aliases matching the Python script.
	cmd.Flags().StringVar(&onlyHost, "oh", "", "alias for --only-host")
	cmd.Flags().StringVar(&excludeHost, "xh", "", "alias for --exclude-host")
	cmd.Flags().MarkHidden("oh")
	cmd.Flags().MarkHidden("xh")

	cmd.MarkFlagsMutuallyExclusive("only", "exclude")
	cmd.MarkFlagsMutuallyExclusive("only-host", "exclude-host")
	cmd.MarkFlagsMutuallyExclusive("oh", "xh")

	return cmd
}

// jsonOutput is the structure written to stdout when --json is used.
type jsonOutput struct {
	PcapName    string            `json:"pcap"`
	Duration    float64           `json:"duration_s"`
	PacketCount int               `json:"packet_count"`
	GRPCCount   int               `json:"grpc_count"`
	FilterDesc  string            `json:"filter,omitempty"`
	Packets     []tshark.Packet   `json:"packets"`
	GRPCCalls   []tshark.GRPCCall `json:"grpc_calls"`
}

func writeJSON(result *analysis.Result) error {
	out := jsonOutput{
		PcapName:    result.PcapName,
		Duration:    result.Duration,
		PacketCount: result.PacketCount,
		GRPCCount:   result.GRPCCount,
		FilterDesc:  result.FilterDesc,
		Packets:     result.Packets,
		GRPCCalls:   result.GRPCCalls,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func buildOptions(only, exclude, onlyHost, excludeHost string, noInterservice bool) analysis.Options {
	return analysis.Options{
		Filter: filter.FilterOptions{
			Only:           splitCSV(only),
			Exclude:        splitCSV(exclude),
			OnlyHosts:      splitCSV(onlyHost),
			ExcludeHosts:   splitCSV(excludeHost),
			NoInterservice: noInterservice,
		},
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

const longDesc = `Analyze a Temporal .pcap capture and produce:
  - <name>_flow.html  — interactive Mermaid diagrams (data flow + sequence)
  - <name>_stats.md   — protocol breakdown, connection matrix, Temporal insights

Output files are written to the same directory as the input pcap.

Protocol names (case-insensitive):
  grpc / http2        Temporal gRPC traffic (ports 7233, 7234, 7235, 7239)
  pgsql / postgresql  PostgreSQL traffic (port 5432)
  http                Temporal UI HTTP traffic (port 8080)
  tcp                 Raw TCP packets
  arp                 ARP packets
  <other>             Matched against tshark's Protocol column

Host specs (--only-host / --exclude-host):
  Container names (e.g. temporal-frontend, hello-world-worker) or raw IPs.
  Matches any packet where the host is the source OR destination.
  Host and protocol filters are ANDed when both are specified.

Examples:
  temporal-analyze capture.pcap
  temporal-analyze capture.pcap --only grpc
  temporal-analyze capture.pcap --only grpc,http
  temporal-analyze capture.pcap --exclude pgsql,tcp
  temporal-analyze capture.pcap --only-host hello-world-worker
  temporal-analyze capture.pcap --exclude-host wireshark,temporal-ui
  temporal-analyze capture.pcap --only grpc --only-host hello-world-worker
  temporal-analyze capture.pcap --no-interservice
  temporal-analyze capture.pcap --no-interservice --only grpc`
