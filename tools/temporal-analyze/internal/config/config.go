package config

import (
	"fmt"
	"regexp"
	"strings"
)

// IPToName maps static container IPs to their human-readable names.
var IPToName = map[string]string{
	"172.20.0.10": "postgresql",
	"172.20.0.21": "temporal-frontend",
	"172.20.0.22": "temporal-history",
	"172.20.0.23": "temporal-matching",
	"172.20.0.24": "temporal-internal-worker",
	"172.20.0.30": "temporal-ui",
	"172.20.0.40": "hello-world-worker",
	"172.20.0.41": "hello-world-starter",
	"172.20.0.42": "scheduled-worker",
	"172.20.0.43": "signals-worker",
	"172.20.0.44": "child-workflows-worker",
	"172.20.0.45": "retries-worker",
	"172.20.0.46": "saga-worker",
	"172.20.0.50": "wireshark",
	"172.20.0.51": "scheduled-starter",
	"172.20.0.52": "signals-starter",
	"172.20.0.53": "signals-approve",
	"172.20.0.54": "signals-reject",
	"172.20.0.55": "child-workflows-starter",
	"172.20.0.56": "retries-starter",
	"172.20.0.57": "saga-starter",
	"172.20.0.58": "saga-fail-starter",
}

// NameToIP is the reverse of IPToName.
var NameToIP = func() map[string]string {
	m := make(map[string]string, len(IPToName))
	for ip, name := range IPToName {
		m[name] = ip
	}
	return m
}()

// PortLabels maps well-known port numbers to human-readable labels.
var PortLabels = map[string]string{
	"7233": "Temporal gRPC (frontend)",
	"7234": "Temporal gRPC (history)",
	"7235": "Temporal gRPC (matching)",
	"7239": "Temporal gRPC (worker)",
	"6933": "Temporal membership (frontend)",
	"6934": "Temporal membership (history)",
	"6935": "Temporal membership (matching)",
	"6939": "Temporal membership (worker)",
	"5432": "PostgreSQL",
	"8080": "HTTP (Temporal UI)",
}

// GRPCPorts is the set of ports that carry Temporal gRPC (HTTP/2) traffic.
var GRPCPorts = map[string]bool{
	"7233": true,
	"7234": true,
	"7235": true,
	"7239": true,
}

// InterserviceHosts are excluded when --no-interservice is set.
var InterserviceHosts = []string{
	"temporal-history",
	"temporal-matching",
	"temporal-internal-worker",
}

// MaxSeqEntries is the maximum compressed rows shown in a sequence diagram.
const MaxSeqEntries = 150

// ProtoPorts maps user-facing protocol filter names to the ports they cover.
var ProtoPorts = map[string][]string{
	"pgsql":      {"5432"},
	"postgresql": {"5432"},
	"grpc":       {"7233", "7234", "7235", "7239"},
	"http2":      {"7233", "7234", "7235", "7239"},
	"http":       {"8080"},
}

// GRPCProtoNames are the protocol filter names that select gRPC traffic.
var GRPCProtoNames = map[string]bool{"grpc": true, "http2": true}

// Resolve maps a container IP to its name, returning the raw IP if unknown.
func Resolve(ip string) string {
	if name, ok := IPToName[ip]; ok {
		return name
	}
	return ip
}

var nonAlphanumRe = regexp.MustCompile(`[^a-zA-Z0-9]`)

// MermaidID sanitizes a string for use as a Mermaid node identifier.
func MermaidID(name string) string {
	return nonAlphanumRe.ReplaceAllString(name, "_")
}

// FmtBytes converts a byte count to a human-readable string.
func FmtBytes(n int) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(n)
	for _, u := range units[:len(units)-1] {
		if f < 1024 {
			return fmt.Sprintf("%.1f %s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1f TB", f)
}

// FmtNum formats an integer with comma separators.
func FmtNum(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	offset := len(s) % 3
	if offset > 0 {
		b.WriteString(s[:offset])
	}
	for i := offset; i < len(s); i += 3 {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
