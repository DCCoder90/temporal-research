package diagram

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"temporal-analyze/internal/config"
	"temporal-analyze/internal/tshark"
)

var parenRe = regexp.MustCompile(`\s*\([^)]*\)`)

// sanitizeEdgeLabel removes parenthetical suffixes (e.g. "(frontend)") from
// labels used inside Mermaid flowchart edge declarations. Mermaid's lexer
// mis-parses parentheses in |"..."|  edge labels as node-shape tokens even
// when they are inside double quotes, producing "Syntax error in text".
func sanitizeEdgeLabel(s string) string {
	return parenRe.ReplaceAllString(s, "")
}

// BuildFlowDiagram builds a Mermaid flowchart LR showing directed traffic.
// PostgreSQL is rendered as a cylinder; all others as stadiums.
func BuildFlowDiagram(packets []tshark.Packet) string {
	type edgeKey struct{ src, dst, label string }
	type edgeVal struct{ pkts, bytes int }

	edges := make(map[edgeKey]*edgeVal)
	nodes := make(map[string]struct{})

	for _, p := range packets {
		src := config.Resolve(p.Src)
		dst := config.Resolve(p.Dst)
		label := config.PortLabels[p.Dport]
		if label == "" {
			if p.Dport != "" {
				label = "TCP:" + p.Dport
			} else {
				label = "TCP"
			}
		}
		key := edgeKey{src, dst, label}
		if edges[key] == nil {
			edges[key] = &edgeVal{}
		}
		edges[key].pkts++
		edges[key].bytes += p.Len
		nodes[src] = struct{}{}
		nodes[dst] = struct{}{}
	}

	// Sort nodes for deterministic output.
	sortedNodes := make([]string, 0, len(nodes))
	for n := range nodes {
		sortedNodes = append(sortedNodes, n)
	}
	sort.Strings(sortedNodes)

	var b strings.Builder
	b.WriteString("flowchart LR\n")

	for _, node := range sortedNodes {
		nid := config.MermaidID(node)
		if node == "postgresql" {
			fmt.Fprintf(&b, "    %s[(\"%s\")]\n", nid, node)
		} else {
			fmt.Fprintf(&b, "    %s([\"%s\"])\n", nid, node)
		}
	}
	b.WriteString("\n")

	// Sort edges by packet count descending.
	type edgeEntry struct {
		key edgeKey
		val edgeVal
	}
	sorted := make([]edgeEntry, 0, len(edges))
	for k, v := range edges {
		sorted = append(sorted, edgeEntry{k, *v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].val.pkts > sorted[j].val.pkts
	})

	for _, e := range sorted {
		sid := config.MermaidID(e.key.src)
		did := config.MermaidID(e.key.dst)
		edgeLabel := fmt.Sprintf("%s — %s pkts / %s",
			sanitizeEdgeLabel(e.key.label), config.FmtNum(e.val.pkts), config.FmtBytes(e.val.bytes))
		fmt.Fprintf(&b, "    %s -->|\"%s\"| %s\n", sid, edgeLabel, did)
	}

	return strings.TrimRight(b.String(), "\n")
}

// BuildSequenceDiagram builds a gRPC-only Mermaid sequenceDiagram.
// Consecutive identical (src, dst, method) runs are compressed into (xN) annotations.
func BuildSequenceDiagram(calls []tshark.GRPCCall) string {
	if len(calls) == 0 {
		ports := []string{"7233", "7234", "7235", "7239"}
		var portArgs []string
		for _, p := range ports {
			portArgs = append(portArgs, fmt.Sprintf("-d tcp.port==%s,http2", p))
		}
		return fmt.Sprintf(
			"sequenceDiagram\n"+
				"    participant tf as temporal-frontend\n"+
				"    Note over tf: No gRPC calls decoded in this capture\n"+
				"    Note over tf: Run tshark -r file.pcap %s -Y http2.headers.path",
			strings.Join(portArgs, " "),
		)
	}

	// Collect participants in first-appearance order.
	seen := make(map[string]struct{})
	var participants []string
	for _, c := range calls {
		if _, ok := seen[c.Src]; !ok {
			seen[c.Src] = struct{}{}
			participants = append(participants, c.Src)
		}
		if _, ok := seen[c.Dst]; !ok {
			seen[c.Dst] = struct{}{}
			participants = append(participants, c.Dst)
		}
	}

	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	for _, p := range participants {
		fmt.Fprintf(&b, "    participant %s as %s\n", config.MermaidID(p), p)
	}
	b.WriteString("\n")

	compressed := compress(func(yield func(string, string, string)) {
		for _, c := range calls {
			yield(c.Src, c.Dst, c.Method)
		}
	})
	writeCompressed(&b, compressed, participants[0])
	return strings.TrimRight(b.String(), "\n")
}

// BuildTrafficSequenceDiagram builds a mixed-protocol Mermaid sequenceDiagram.
// gRPC port packets are replaced by the richer method-name entries from calls.
func BuildTrafficSequenceDiagram(packets []tshark.Packet, calls []tshark.GRPCCall) string {
	type event struct {
		t     float64
		src   string
		dst   string
		label string
	}

	var events []event

	for _, p := range packets {
		if config.GRPCPorts[p.Dport] || config.GRPCPorts[p.Sport] {
			continue // replaced by grpc call entries
		}
		src := config.Resolve(p.Src)
		dst := config.Resolve(p.Dst)
		label := p.Proto
		if label == "" {
			if l, ok := config.PortLabels[p.Dport]; ok {
				label = l
			} else if p.Dport != "" {
				label = "TCP:" + p.Dport
			} else {
				label = "TCP"
			}
		}
		events = append(events, event{p.T, src, dst, label})
	}

	for _, c := range calls {
		events = append(events, event{c.T, c.Src, c.Dst, c.Method})
	}

	// Sort by timestamp.
	sort.Slice(events, func(i, j int) bool { return events[i].t < events[j].t })

	if len(events) == 0 {
		return "sequenceDiagram\n" +
			"    participant tf as temporal-frontend\n" +
			"    Note over tf: No traffic decoded in this capture"
	}

	// Collect participants in first-appearance order.
	seen := make(map[string]struct{})
	var participants []string
	for _, e := range events {
		if _, ok := seen[e.src]; !ok {
			seen[e.src] = struct{}{}
			participants = append(participants, e.src)
		}
		if _, ok := seen[e.dst]; !ok {
			seen[e.dst] = struct{}{}
			participants = append(participants, e.dst)
		}
	}

	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	for _, p := range participants {
		fmt.Fprintf(&b, "    participant %s as %s\n", config.MermaidID(p), p)
	}
	b.WriteString("\n")

	compressed := compress(func(yield func(string, string, string)) {
		for _, e := range events {
			yield(e.src, e.dst, e.label)
		}
	})
	writeCompressed(&b, compressed, participants[0])
	return strings.TrimRight(b.String(), "\n")
}

// compressedRow is a run-length-compressed sequence entry.
type compressedRow struct {
	src, dst, label string
	count           int
}

// compress collapses consecutive identical (src, dst, label) runs.
func compress(iter func(yield func(string, string, string))) []compressedRow {
	var rows []compressedRow
	iter(func(src, dst, label string) {
		if len(rows) > 0 && rows[len(rows)-1].src == src &&
			rows[len(rows)-1].dst == dst && rows[len(rows)-1].label == label {
			rows[len(rows)-1].count++
		} else {
			rows = append(rows, compressedRow{src, dst, label, 1})
		}
	})
	return rows
}

func writeCompressed(b *strings.Builder, rows []compressedRow, firstParticipant string) {
	total := len(rows)
	limit := rows
	if total > config.MaxSeqEntries {
		limit = rows[:config.MaxSeqEntries]
	}
	for _, r := range limit {
		sid := config.MermaidID(r.src)
		did := config.MermaidID(r.dst)
		display := r.label
		if r.count > 1 {
			display = fmt.Sprintf("%s (x%s)", r.label, config.FmtNum(r.count))
		}
		fmt.Fprintf(b, "    %s->>%s: %s\n", sid, did, display)
	}
	if total > config.MaxSeqEntries {
		omitted := total - config.MaxSeqEntries
		fmt.Fprintf(b, "    Note over %s: ... %s more event types not shown — limit %d\n",
			config.MermaidID(firstParticipant), config.FmtNum(omitted), config.MaxSeqEntries)
	}
}
