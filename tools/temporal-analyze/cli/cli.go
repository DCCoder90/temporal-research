package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"temporal-analyze/internal/analysis"
	"temporal-analyze/internal/config"
	"temporal-analyze/internal/filter"

	"github.com/spf13/cobra"
)

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
	var noInterservice bool

	cmd := &cobra.Command{
		Use:   "temporal-analyze <pcap-file>",
		Short: "Analyze a Temporal .pcap capture and produce flow diagrams and statistics.",
		Long:  longDesc,
		Args:  cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := buildOptions(only, exclude, onlyHost, excludeHost, noInterservice)

			filterDesc := filter.Describe(opts.Filter)
			fmt.Fprintf(os.Stderr, "Analyzing %s ...\n", args[0])
			if filterDesc != "" {
				fmt.Fprintf(os.Stderr, "  Filter: %s\n", filterDesc)
			}

			fmt.Fprintln(os.Stderr, "  [1/2] Extracting packet data and decoding gRPC calls ...")
			result, err := analysis.Run(args[0], opts)
			if err != nil {
				if errors.Is(err, analysis.ErrNoPacketsAfterFilter) {
					return fmt.Errorf("no packets remain after filtering — try relaxing the filter")
				}
				return err
			}

			fmt.Fprintf(os.Stderr, "        %s packets  |  %s  |  %.1fs window  |  %s gRPC calls\n",
				config.FmtNum(result.PacketCount),
				config.FmtBytes(result.TotalBytes),
				result.Duration,
				config.FmtNum(result.GRPCCount),
			)

			fmt.Fprintln(os.Stderr, "  [2/2] Writing output files ...")
			paths, err := analysis.WriteResult(args[0], result)
			if err != nil {
				return err
			}

			fmt.Fprintln(os.Stderr, "\n✓  Done.")
			fmt.Fprintf(os.Stderr, "   Flow diagram : %s\n", paths[0])
			fmt.Fprintf(os.Stderr, "   Statistics   : %s\n", paths[1])
			fmt.Fprintln(os.Stderr)
			return nil
		},
	}

	cmd.Flags().StringVarP(&only, "only", "o", "", "comma-separated protocols to include (all others hidden)")
	cmd.Flags().StringVarP(&exclude, "exclude", "x", "", "comma-separated protocols to exclude")
	cmd.Flags().StringVar(&onlyHost, "only-host", "", "comma-separated hosts (names or IPs) to include")
	cmd.Flags().StringVar(&excludeHost, "exclude-host", "", "comma-separated hosts to exclude")
	cmd.Flags().BoolVar(&noInterservice, "no-interservice", false,
		"exclude temporal-history, temporal-matching, temporal-internal-worker")

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
