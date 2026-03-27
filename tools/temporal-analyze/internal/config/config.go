package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// IPToName maps container IPs to human-readable names.
// Pre-populated with defaults so that tests that do not call Load() continue to work.
// Load() replaces this map with data from config.json.
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
var NameToIP = reverseMap(IPToName)

// PortLabels maps well-known port numbers to human-readable labels.
// Pre-populated with defaults; replaced by Load().
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
// These are Temporal protocol constants and are not user-configurable.
var GRPCPorts = map[string]bool{
	"7233": true,
	"7234": true,
	"7235": true,
	"7239": true,
}

// InterserviceHosts are excluded when --no-interservice is set.
// These are matched against the name column of config.json.
var InterserviceHosts = []string{
	"temporal-history",
	"temporal-matching",
	"temporal-internal-worker",
}

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

// MaxSeqEntries is the maximum compressed rows shown in a sequence diagram page.
const MaxSeqEntries = 150

// configFile is the structure of config.json.
type configFile struct {
	Hosts map[string]string `json:"hosts"`
	Ports map[string]string `json:"ports"`
}

// Load reads config.json from the first directory in ConfigSearchDirs() that
// contains it. Returns a descriptive error if the file is missing or cannot be
// parsed. Must be called at program startup before any analysis runs.
func Load() error {
	dirs := ConfigSearchDirs()

	cfgPath := findFile(dirs, "config.json")
	if cfgPath == "" {
		return fmt.Errorf(
			"config.json not found\n\nSearched:\n%s\n\nPlace config.json in one of these directories.\nA default config.json is included in each release archive.",
			formatDirs(dirs),
		)
	}

	cfg, err := readConfigJSON(cfgPath)
	if err != nil {
		return err
	}

	IPToName = cfg.Hosts
	NameToIP = reverseMap(cfg.Hosts)
	PortLabels = cfg.Ports
	return nil
}

// ConfigSearchDirs returns the directories searched for config.json,
// in priority order: executable directory first, then ~/.config/temporal-analyze/.
// Useful for printing helpful error messages to the user.
func ConfigSearchDirs() []string {
	var dirs []string

	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}

	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "temporal-analyze"))
	}

	return dirs
}

// ── JSON loader ───────────────────────────────────────────────────────────────

func readConfigJSON(path string) (*configFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	var cfg configFile
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("%s: \"hosts\" map is empty or missing", path)
	}
	if len(cfg.Ports) == 0 {
		return nil, fmt.Errorf("%s: \"ports\" map is empty or missing", path)
	}
	return &cfg, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func findFile(dirs []string, name string) string {
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func formatDirs(dirs []string) string {
	lines := make([]string, len(dirs))
	for i, d := range dirs {
		lines[i] = "  " + d
	}
	return strings.Join(lines, "\n")
}

func reverseMap(m map[string]string) map[string]string {
	r := make(map[string]string, len(m))
	for k, v := range m {
		r[v] = k
	}
	return r
}

// ── Public utilities ──────────────────────────────────────────────────────────

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
