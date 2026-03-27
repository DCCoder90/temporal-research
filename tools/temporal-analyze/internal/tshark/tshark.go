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
	T          float64
	Src        string
	Dst        string
	Sport      string
	Dport      string
	Len        int     // frame.len — total frame bytes including headers
	Proto      string
	TCPStream  int     // tcp.stream index; -1 if not TCP
	TCPLen     int     // tcp.len — payload bytes only; 0 if not TCP
	TCPFlags   int     // tcp.flags as integer (e.g. 0x002 = SYN); 0 if not TCP
	Retransmit bool    // tcp.analysis.retransmission
	RTT        float64 // tcp.analysis.ack_rtt in seconds; 0 if not available
}

// GRPCCall represents a decoded gRPC method call.
type GRPCCall struct {
	T          float64
	Src        string // resolved container name
	Dst        string // resolved container name
	FullPath   string // full :path value e.g. /temporal.api.workflowservice.v1.WorkflowService/PollWorkflowTaskQueue
	Service    string // service portion e.g. temporal.api.workflowservice.v1.WorkflowService
	Method     string // last path segment e.g. PollWorkflowTaskQueue
	TCPStream  int    // tcp.stream index
	StreamID   int    // HTTP/2 stream ID
	StatusCode int    // gRPC status code (0=OK); -1 if unknown
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
		"tcp.stream",
		"tcp.len",
		"tcp.flags",
		"tcp.analysis.retransmission",
		"tcp.analysis.ack_rtt",
	}
	rows, err := RunTshark(pcap, fields, nil, "")
	if err != nil {
		return nil, 0, err
	}

	var packets []Packet
	for _, row := range rows {
		src, dst := row[1], row[2]
		if src == "" || dst == "" {
			continue
		}
		t, err := strconv.ParseFloat(row[0], 64)
		if err != nil {
			continue
		}
		tcpStream := -1
		if row[7] != "" {
			tcpStream = parseInt(row[7])
		}
		packets = append(packets, Packet{
			T:          t,
			Src:        src,
			Dst:        dst,
			Sport:      row[3],
			Dport:      row[4],
			Len:        parseInt(row[5]),
			Proto:      row[6],
			TCPStream:  tcpStream,
			TCPLen:     parseInt(row[8]),
			TCPFlags:   parseHexInt(row[9]),
			Retransmit: row[10] == "1",
			RTT:        parseFloat(row[11]),
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
	fields := []string{
		"frame.time_epoch",
		"ip.src",
		"ip.dst",
		"http2.headers.path",
		"tcp.stream",
		"http2.streamid",
	}

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
		path := row[3]
		if path == "" {
			continue
		}
		t, err := strconv.ParseFloat(row[0], 64)
		if err != nil {
			continue
		}

		// Derive service and method from path e.g. /pkg.ServiceName/MethodName
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		method := parts[len(parts)-1]
		service := ""
		if len(parts) >= 2 {
			service = parts[len(parts)-2]
		}

		calls = append(calls, GRPCCall{
			T:         t,
			Src:       config.Resolve(row[1]),
			Dst:       config.Resolve(row[2]),
			FullPath:  path,
			Service:   service,
			Method:    method,
			TCPStream: parseInt(row[4]),
			StreamID:  parseInt(firstVal(row[5])),
			StatusCode: -1, // filled in below
		})
	}

	// Best-effort: join gRPC status codes from response trailer frames.
	statusMap := extractGRPCStatuses(pcap, decodeArgs)
	for i := range calls {
		key := fmt.Sprintf("%d:%d", calls[i].TCPStream, calls[i].StreamID)
		if code, ok := statusMap[key]; ok {
			calls[i].StatusCode = code
		}
	}

	return calls, nil
}

// extractGRPCStatuses returns a map of "tcpStream:streamID" -> gRPC status code
// by extracting frames that carry grpc.status_code. Best-effort: returns empty
// map on any error so the caller degrades gracefully.
func extractGRPCStatuses(pcap string, decodeArgs []string) map[string]int {
	result := make(map[string]int)

	fields := []string{"tcp.stream", "http2.streamid", "grpc.status_code"}
	rows, err := RunTshark(pcap, fields, decodeArgs, "grpc.status_code")
	if err != nil {
		return result
	}

	for _, row := range rows {
		tcpStream := row[0]
		streamID := firstVal(row[1])
		statusStr := row[2]
		if tcpStream == "" || streamID == "" || statusStr == "" {
			continue
		}
		key := fmt.Sprintf("%s:%s", tcpStream, streamID)
		result[key] = parseInt(statusStr)
	}
	return result
}

// ── Parsing helpers ──────────────────────────────────────────────────────────

func parseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// parseHexInt parses a tshark hex flag value like "0x00000002" → 2.
func parseHexInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.TrimPrefix(s, "0x")
	n, _ := strconv.ParseInt(s, 16, 64)
	return int(n)
}

// firstVal returns the first comma-separated value from a tshark field that
// may contain multiple values (e.g. http2.streamid on a reassembled frame).
func firstVal(s string) string {
	if idx := strings.IndexByte(s, ','); idx >= 0 {
		return s[:idx]
	}
	return s
}
