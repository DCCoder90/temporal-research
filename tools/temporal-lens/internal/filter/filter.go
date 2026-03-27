package filter

import (
	"strings"
	"temporal-lens/internal/config"
	"temporal-lens/internal/tshark"
)

// FilterOptions controls which packets and gRPC calls are retained.
type FilterOptions struct {
	Only           []string
	Exclude        []string
	OnlyHosts      []string
	ExcludeHosts   []string
	NoInterservice bool
}

// Apply filters packets and grpc calls. Protocol and host filters are ANDed.
// Three phases: protocol filter → host filter → interservice filter.
func Apply(packets []tshark.Packet, calls []tshark.GRPCCall, opts FilterOptions) ([]tshark.Packet, []tshark.GRPCCall) {
	// ── Protocol filter ────────────────────────────────────────────────────────
	if len(opts.Only) > 0 {
		protos := lowerAll(opts.Only)
		packets = filterPackets(packets, func(p tshark.Packet) bool {
			for _, pr := range protos {
				if MatchesProtocol(p, pr) {
					return true
				}
			}
			return false
		})
		if !anyGRPCProto(protos) {
			calls = nil
		}
	} else if len(opts.Exclude) > 0 {
		protos := lowerAll(opts.Exclude)
		packets = filterPackets(packets, func(p tshark.Packet) bool {
			for _, pr := range protos {
				if MatchesProtocol(p, pr) {
					return false
				}
			}
			return true
		})
		if anyGRPCProto(protos) {
			calls = nil
		}
	}

	// ── Host filter ────────────────────────────────────────────────────────────
	if len(opts.OnlyHosts) > 0 {
		hostIPs, hostNames := parseHostSpecs(opts.OnlyHosts)
		packets = filterPackets(packets, func(p tshark.Packet) bool {
			return hostIPs[p.Src] || hostIPs[p.Dst]
		})
		calls = filterCalls(calls, func(c tshark.GRPCCall) bool {
			return hostNames[c.Src] || hostNames[c.Dst]
		})
	} else if len(opts.ExcludeHosts) > 0 {
		hostIPs, hostNames := parseHostSpecs(opts.ExcludeHosts)
		packets = filterPackets(packets, func(p tshark.Packet) bool {
			return !hostIPs[p.Src] && !hostIPs[p.Dst]
		})
		calls = filterCalls(calls, func(c tshark.GRPCCall) bool {
			return !hostNames[c.Src] && !hostNames[c.Dst]
		})
	}

	// ── Inter-service filter ────────────────────────────────────────────────────
	if opts.NoInterservice {
		isIPs, isNames := parseHostSpecs(config.InterserviceHosts)
		packets = filterPackets(packets, func(p tshark.Packet) bool {
			return !isIPs[p.Src] && !isIPs[p.Dst]
		})
		calls = filterCalls(calls, func(c tshark.GRPCCall) bool {
			return !isNames[c.Src] && !isNames[c.Dst]
		})
	}

	return packets, calls
}

// MatchesProtocol returns true if a packet belongs to the named protocol.
// Named aliases (grpc, pgsql, http2, http, postgresql) are matched by port.
// Anything else is matched against tshark's Protocol column.
func MatchesProtocol(p tshark.Packet, proto string) bool {
	key := strings.ToLower(proto)
	if ports, ok := config.ProtoPorts[key]; ok {
		portSet := make(map[string]bool, len(ports))
		for _, port := range ports {
			portSet[port] = true
		}
		return portSet[p.Dport] || portSet[p.Sport]
	}
	return strings.ToLower(p.Proto) == key
}

// ShowTrafficSeq returns false when the only-filter selects only gRPC protocols,
// since the gRPC sequence diagram would be identical to the traffic sequence diagram.
func ShowTrafficSeq(opts FilterOptions) bool {
	if len(opts.Only) == 0 {
		return true
	}
	for _, p := range opts.Only {
		if !config.GRPCProtoNames[strings.ToLower(p)] {
			return true
		}
	}
	return false
}

// Describe returns a human-readable description of the active filters.
func Describe(opts FilterOptions) string {
	var parts []string
	if len(opts.Only) > 0 {
		parts = append(parts, "only: "+strings.Join(opts.Only, ", "))
	} else if len(opts.Exclude) > 0 {
		parts = append(parts, "exclude: "+strings.Join(opts.Exclude, ", "))
	}
	if len(opts.OnlyHosts) > 0 {
		parts = append(parts, "host: "+strings.Join(opts.OnlyHosts, ", "))
	} else if len(opts.ExcludeHosts) > 0 {
		parts = append(parts, "exclude-host: "+strings.Join(opts.ExcludeHosts, ", "))
	}
	if opts.NoInterservice {
		parts = append(parts, "no-interservice")
	}
	return strings.Join(parts, " | ")
}

// parseHostSpecs resolves a list of container names or IPs into (ipSet, nameSet).
func parseHostSpecs(specs []string) (map[string]bool, map[string]bool) {
	ips := make(map[string]bool)
	names := make(map[string]bool)
	for _, spec := range specs {
		if ip, ok := config.NameToIP[spec]; ok {
			ips[ip] = true
			names[spec] = true
		} else if name, ok := config.IPToName[spec]; ok {
			ips[spec] = true
			names[name] = true
		} else {
			ips[spec] = true
			names[spec] = true
		}
	}
	return ips, names
}

func lowerAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToLower(s)
	}
	return out
}

func anyGRPCProto(protos []string) bool {
	for _, p := range protos {
		if config.GRPCProtoNames[p] {
			return true
		}
	}
	return false
}

func filterPackets(packets []tshark.Packet, keep func(tshark.Packet) bool) []tshark.Packet {
	out := packets[:0:0]
	for _, p := range packets {
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

func filterCalls(calls []tshark.GRPCCall, keep func(tshark.GRPCCall) bool) []tshark.GRPCCall {
	out := calls[:0:0]
	for _, c := range calls {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}
