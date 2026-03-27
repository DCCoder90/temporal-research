package tshark

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"temporal-analyze/internal/config"
)

// Packet represents a single IP packet from the capture.
type Packet struct {
	T     float64
	Src   string
	Dst   string
	Sport string
	Dport string
	Len   int
	Proto string
}

// GRPCCall represents a decoded gRPC method call.
type GRPCCall struct {
	T      float64
	Src    string // resolved container name
	Dst    string // resolved container name
	Method string // last path segment of the :path header
}

// RunTshark executes tshark and returns tab-separated field rows.
// Short rows are padded to len(fields) so callers can always unpack.
func RunTshark(pcap string, fields []string, extraArgs []string, filterExpr string) ([][]string, error) {
	if _, err := exec.LookPath("tshark"); err != nil {
		return nil, fmt.Errorf("tshark not found in PATH: %w", err)
	}

	args := []string{"-r", pcap, "-T", "fields"}
	args = append(args, extraArgs...)
	for _, f := range fields {
		args = append(args, "-e", f)
	}
	args = append(args, "-E", "separator=\t", "-E", "header=n")
	if filterExpr != "" {
		args = append(args, "-Y", filterExpr)
	}

	out, err := exec.Command("tshark", args...).Output()
	if err != nil {
		// tshark exits non-zero for many benign reasons (dissector warnings, etc.)
		// Only treat it as an error if there is no output at all.
		if len(out) == 0 {
			return nil, fmt.Errorf("tshark failed: %w", err)
		}
	}

	var rows [][]string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		for len(parts) < len(fields) {
			parts = append(parts, "")
		}
		rows = append(rows, parts[:len(fields)])
	}
	return rows, nil
}

// ExtractPackets returns all IP packets and the capture duration in seconds.
func ExtractPackets(pcap string) ([]Packet, float64, error) {
	fields := []string{
		"frame.time_epoch",
		"ip.src",
		"ip.dst",
		"tcp.srcport",
		"tcp.dstport",
		"frame.len",
		"_ws.col.Protocol",
	}
	rows, err := RunTshark(pcap, fields, nil, "")
	if err != nil {
		return nil, 0, err
	}

	var packets []Packet
	for _, row := range rows {
		timeS, src, dst, sport, dport, length, proto := row[0], row[1], row[2], row[3], row[4], row[5], row[6]
		if src == "" || dst == "" {
			continue
		}
		t, err := strconv.ParseFloat(timeS, 64)
		if err != nil {
			continue
		}
		l, _ := strconv.Atoi(length)
		packets = append(packets, Packet{
			T:     t,
			Src:   src,
			Dst:   dst,
			Sport: sport,
			Dport: dport,
			Len:   l,
			Proto: proto,
		})
	}

	if len(packets) == 0 {
		return nil, 0, nil
	}

	minT, maxT := packets[0].T, packets[0].T
	for _, p := range packets[1:] {
		if p.T < minT {
			minT = p.T
		}
		if p.T > maxT {
			maxT = p.T
		}
	}
	return packets, maxT - minT, nil
}

// ExtractGRPCCalls returns decoded gRPC method calls from HTTP/2 HEADERS frames.
// Uses -d tcp.port==PORT,http2 overrides for all Temporal gRPC ports.
func ExtractGRPCCalls(pcap string) ([]GRPCCall, error) {
	fields := []string{"frame.time_epoch", "ip.src", "ip.dst", "http2.headers.path"}

	// Build decode-as args for all gRPC ports.
	var decodeArgs []string
	ports := []string{"7233", "7234", "7235", "7239"}
	for _, port := range ports {
		decodeArgs = append(decodeArgs, "-d", fmt.Sprintf("tcp.port==%s,http2", port))
	}

	rows, err := RunTshark(pcap, fields, decodeArgs, "http2.headers.path")
	if err != nil {
		return nil, err
	}

	var calls []GRPCCall
	for _, row := range rows {
		timeS, src, dst, path := row[0], row[1], row[2], row[3]
		if path == "" {
			continue
		}
		t, err := strconv.ParseFloat(timeS, 64)
		if err != nil {
			continue
		}
		// Last segment of path is the method name.
		parts := strings.Split(path, "/")
		method := parts[len(parts)-1]
		calls = append(calls, GRPCCall{
			T:      t,
			Src:    config.Resolve(src),
			Dst:    config.Resolve(dst),
			Method: method,
		})
	}
	return calls, nil
}
