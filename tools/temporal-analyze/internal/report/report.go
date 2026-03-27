package report

import (
	"fmt"
	"sort"
	"strings"
	"temporal-analyze/internal/config"
	"temporal-analyze/internal/tshark"
	"time"
)

// HTMLInput holds all data needed to render the HTML report.
type HTMLInput struct {
	PcapName    string
	Duration    float64
	Packets     []tshark.Packet
	GRPCCalls   []tshark.GRPCCall
	FlowDiagram string
	SeqDiagram  string
	TrafficSeq  *string // nil = omit the traffic section
	FilterDesc  string
}

// GenerateHTML returns the complete HTML report as a string.
func GenerateHTML(in HTMLInput) string {
	totalPkts := len(in.Packets)
	totalBytes := 0
	for _, p := range in.Packets {
		totalBytes += p.Len
	}

	filterBadge := ""
	if in.FilterDesc != "" {
		filterBadge = fmt.Sprintf(`<span style="background:#fff3cd;color:#856404;"><strong>Filter:</strong> %s</span>`, in.FilterDesc)
	}

	trafficSeqSection := ""
	grpcIdx := 1
	if in.TrafficSeq != nil {
		trafficSeqSection = "\n  <h2>Traffic Sequence Diagram</h2>\n" +
			"  <p class=\"hint\">All protocol traffic in chronological order." +
			" gRPC arrows show the RPC method name; others show the protocol label." +
			" Consecutive identical events are compressed (xN).</p>\n" +
			"  <div class=\"card\">\n" +
			"    <div class=\"card-toolbar\">\n" +
			"      <button onclick=\"zoomIn(1)\">&#xFF0B; Zoom in</button>\n" +
			"      <button onclick=\"zoomOut(1)\">&#xFF0D; Zoom out</button>\n" +
			"      <button onclick=\"resetZoom(1)\">&#x27F3; Reset</button>\n" +
			"      <span class=\"zoom-hint\">Scroll to zoom &nbsp;&middot;&nbsp; Drag to pan</span>\n" +
			"    </div>\n" +
			"    <div class=\"diagram-wrap\">\n" +
			"      <div class=\"mermaid\">\n" +
			*in.TrafficSeq + "\n" +
			"      </div>\n" +
			"    </div>\n" +
			"  </div>"
		grpcIdx = 2
	}

	return fmt.Sprintf(htmlTemplate,
		in.PcapName,
		in.PcapName,
		time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf("%.1fs", in.Duration),
		config.FmtNum(totalPkts),
		config.FmtBytes(totalBytes),
		config.FmtNum(len(in.GRPCCalls)),
		filterBadge,
		in.FlowDiagram,
		trafficSeqSection,
		grpcIdx,
		grpcIdx,
		grpcIdx,
		in.SeqDiagram,
	)
}

// GenerateStats returns the Markdown statistics report as a string.
func GenerateStats(pcapName string, packets []tshark.Packet, calls []tshark.GRPCCall, duration float64, filterDesc string) string {
	totalPkts := len(packets)
	totalBytes := 0
	for _, p := range packets {
		totalBytes += p.Len
	}

	var b strings.Builder

	filterVal := "_none (all protocols)_"
	if filterDesc != "" {
		filterVal = "`" + filterDesc + "`"
	}

	b.WriteString("# Temporal Traffic Analysis\n\n")
	table(&b, []string{"", ""},
		[][]string{
			{"**File**", "`" + pcapName + "`"},
			{"**Generated**", time.Now().Format("2006-01-02 15:04:05")},
			{"**Capture Duration**", fmt.Sprintf("%.1f seconds", duration)},
			{"**Filter**", filterVal},
			{"**Total Packets**", config.FmtNum(totalPkts)},
			{"**Total Bytes**", config.FmtBytes(totalBytes)},
			{"**gRPC Calls Decoded**", config.FmtNum(len(calls))},
		},
	)
	b.WriteString("---\n")

	// Protocol breakdown
	heading(&b, 2, "Protocol Breakdown")
	type protoBucket struct {
		label string
		pkts  int
		bytes int
	}
	protoMap := make(map[string]*protoBucket)
	for _, p := range packets {
		label := config.PortLabels[p.Dport]
		if label == "" {
			label = p.Proto
			if label == "" {
				label = "Other"
			}
		}
		if protoMap[label] == nil {
			protoMap[label] = &protoBucket{label: label}
		}
		protoMap[label].pkts++
		protoMap[label].bytes += p.Len
	}
	protoSlice := make([]*protoBucket, 0, len(protoMap))
	for _, v := range protoMap {
		protoSlice = append(protoSlice, v)
	}
	sort.Slice(protoSlice, func(i, j int) bool { return protoSlice[i].pkts > protoSlice[j].pkts })
	protoRows := make([][]string, len(protoSlice))
	for i, b := range protoSlice {
		pct := float64(0)
		if totalPkts > 0 {
			pct = 100 * float64(b.pkts) / float64(totalPkts)
		}
		protoRows[i] = []string{b.label, config.FmtNum(b.pkts), config.FmtBytes(b.bytes), fmt.Sprintf("%.1f%%", pct)}
	}
	table(&b, []string{"Protocol", "Packets", "Bytes", "% of Traffic"}, protoRows)
	b.WriteString("---\n")

	// Connection matrix
	heading(&b, 2, "Connection Matrix")
	b.WriteString("_Top 30 directed connections by packet count._\n\n")
	type connKey struct{ src, dst, label string }
	type connVal struct{ pkts, bytes int }
	connMap := make(map[connKey]*connVal)
	for _, p := range packets {
		src := config.Resolve(p.Src)
		dst := config.Resolve(p.Dst)
		lbl := config.PortLabels[p.Dport]
		if lbl == "" {
			lbl = p.Dport
			if lbl == "" {
				lbl = "?"
			}
		}
		k := connKey{src, dst, lbl}
		if connMap[k] == nil {
			connMap[k] = &connVal{}
		}
		connMap[k].pkts++
		connMap[k].bytes += p.Len
	}
	type connEntry struct {
		key connKey
		val connVal
	}
	connSlice := make([]connEntry, 0, len(connMap))
	for k, v := range connMap {
		connSlice = append(connSlice, connEntry{k, *v})
	}
	sort.Slice(connSlice, func(i, j int) bool { return connSlice[i].val.pkts > connSlice[j].val.pkts })
	if len(connSlice) > 30 {
		connSlice = connSlice[:30]
	}
	connRows := make([][]string, len(connSlice))
	for i, e := range connSlice {
		connRows[i] = []string{e.key.src, e.key.dst, e.key.label, config.FmtNum(e.val.pkts), config.FmtBytes(e.val.bytes)}
	}
	table(&b, []string{"Source", "Destination", "Protocol / Port", "Packets", "Bytes"}, connRows)
	b.WriteString("---\n")

	// gRPC method calls
	heading(&b, 2, "gRPC Method Calls")
	if len(calls) == 0 {
		ports := []string{"7233", "7234", "7235", "7239"}
		var portArgs []string
		for _, p := range ports {
			portArgs = append(portArgs, fmt.Sprintf("-d tcp.port==%s,http2", p))
		}
		b.WriteString("> **No gRPC calls were decoded.** This can happen when tshark's HTTP/2 dissector\n")
		b.WriteString("> did not recognise traffic on port 7233.  Verify with:\n")
		b.WriteString("> ```\n")
		fmt.Fprintf(&b, "> tshark -r <file> %s -Y http2.headers.path\n", strings.Join(portArgs, " "))
		b.WriteString("> ```\n")
	} else {
		b.WriteString("_All methods decoded from HTTP/2 HEADERS on Temporal gRPC ports (7233, 7234, 7235, 7239)._\n\n")

		type methodData struct {
			count   int
			sources map[string]struct{}
		}
		methodMap := make(map[string]*methodData)
		for _, c := range calls {
			if methodMap[c.Method] == nil {
				methodMap[c.Method] = &methodData{sources: make(map[string]struct{})}
			}
			methodMap[c.Method].count++
			methodMap[c.Method].sources[c.Src] = struct{}{}
		}
		type methodEntry struct {
			method string
			data   *methodData
		}
		methods := make([]methodEntry, 0, len(methodMap))
		for k, v := range methodMap {
			methods = append(methods, methodEntry{k, v})
		}
		sort.Slice(methods, func(i, j int) bool { return methods[i].data.count > methods[j].data.count })
		methodRows := make([][]string, len(methods))
		for i, m := range methods {
			srcs := make([]string, 0, len(m.data.sources))
			for s := range m.data.sources {
				srcs = append(srcs, s)
			}
			sort.Strings(srcs)
			methodRows[i] = []string{"`" + m.method + "`", config.FmtNum(m.data.count), strings.Join(srcs, ", ")}
		}
		table(&b, []string{"Method", "Calls", "Called By"}, methodRows)
		b.WriteString("---\n")

		// Build method count lookup
		mc := make(map[string]int, len(methods))
		for _, m := range methods {
			mc[m.method] = m.data.count
		}

		heading(&b, 2, "Temporal-Specific Insights")

		heading(&b, 3, "Workflow Lifecycle")
		table(&b, []string{"Event", "Count", "Notes"}, [][]string{
			{"`StartWorkflowExecution`", config.FmtNum(mc["StartWorkflowExecution"]), "new workflow runs initiated"},
			{"`SignalWorkflowExecution`", config.FmtNum(mc["SignalWorkflowExecution"]), "signals delivered to running workflows"},
			{"`QueryWorkflow`", config.FmtNum(mc["QueryWorkflow"]), "synchronous query calls"},
			{"`RequestCancelWorkflowExecution`", config.FmtNum(mc["RequestCancelWorkflowExecution"]), "graceful cancellation requests"},
			{"`TerminateWorkflowExecution`", config.FmtNum(mc["TerminateWorkflowExecution"]), "forced termination"},
			{"`GetWorkflowExecutionHistory`", config.FmtNum(mc["GetWorkflowExecutionHistory"]), "history fetches (UI or SDK)"},
		})

		heading(&b, 3, "Task Queue Activity")
		wfPolls := mc["PollWorkflowTaskQueue"]
		wfDone := mc["RespondWorkflowTaskCompleted"]
		actPolls := mc["PollActivityTaskQueue"]
		actDone := mc["RespondActivityTaskCompleted"]
		actFailed := mc["RespondActivityTaskFailed"]
		actHB := mc["RecordActivityTaskHeartbeat"]
		table(&b, []string{"Metric", "Count", "Notes"}, [][]string{
			{"`PollWorkflowTaskQueue`", config.FmtNum(wfPolls), "long-poll; returns when a workflow task is ready"},
			{"`RespondWorkflowTaskCompleted`", config.FmtNum(wfDone), "worker finished executing a workflow task"},
			{"`PollActivityTaskQueue`", config.FmtNum(actPolls), "long-poll; returns when an activity task is ready"},
			{"`RespondActivityTaskCompleted`", config.FmtNum(actDone), "activity returned successfully"},
			{"`RespondActivityTaskFailed`", config.FmtNum(actFailed), "activity returned an error — Temporal will retry"},
			{"`RecordActivityTaskHeartbeat`", config.FmtNum(actHB), "long-running activity progress ping"},
		})

		totalPolls := wfPolls + actPolls
		totalDone := wfDone + actDone
		if totalPolls > 0 && totalDone > 0 {
			heading(&b, 3, "Worker Efficiency")
			ratio := float64(totalPolls) / float64(totalDone)
			note := "workers are executing at high throughput"
			if ratio > 50 {
				note = "expected for idle long-poll workers"
			}
			fmt.Fprintf(&b, "- Total poll calls: **%s**\n", config.FmtNum(totalPolls))
			fmt.Fprintf(&b, "- Total task completions: **%s**\n", config.FmtNum(totalDone))
			fmt.Fprintf(&b, "- Poll-to-completion ratio: **%.0f:1** — %s\n\n", ratio, note)
		}

		if actFailed > 0 {
			heading(&b, 3, "Activity Retry Analysis")
			fmt.Fprintf(&b, "- **%s** activity failure(s) observed (RespondActivityTaskFailed).\n", config.FmtNum(actFailed))
			b.WriteString("  Temporal will schedule retries automatically per each workflow's RetryPolicy.\n")
			if actDone > 0 {
				failRate := 100 * float64(actFailed) / float64(actFailed+actDone)
				fmt.Fprintf(&b, "- **%s** eventual completion(s) recorded.\n", config.FmtNum(actDone))
				fmt.Fprintf(&b, "- Observed failure rate: **%.1f%%** of activity attempts.\n", failRate)
			}
			b.WriteString("\n")
		}

		heading(&b, 3, "Schedule Management")
		table(&b, []string{"Operation", "Count"}, [][]string{
			{"`CreateSchedule`", config.FmtNum(mc["CreateSchedule"])},
			{"`UpdateSchedule`", config.FmtNum(mc["UpdateSchedule"])},
			{"`DeleteSchedule`", config.FmtNum(mc["DeleteSchedule"])},
			{"`ListSchedules`", config.FmtNum(mc["ListSchedules"])},
		})

		heading(&b, 3, "Namespace and Cluster")
		table(&b, []string{"Operation", "Count"}, [][]string{
			{"`RegisterNamespace`", config.FmtNum(mc["RegisterNamespace"])},
			{"`DescribeNamespace`", config.FmtNum(mc["DescribeNamespace"])},
			{"`GetClusterInfo`", config.FmtNum(mc["GetClusterInfo"])},
			{"`GetSystemInfo`", config.FmtNum(mc["GetSystemInfo"])},
		})
		b.WriteString("---\n")
	}

	// Top talkers
	heading(&b, 2, "Top Talkers")
	sent := make(map[string]int)
	recv := make(map[string]int)
	for _, p := range packets {
		src := config.Resolve(p.Src)
		dst := config.Resolve(p.Dst)
		sent[src]++
		recv[dst]++
	}
	allHosts := make(map[string]struct{})
	for h := range sent {
		allHosts[h] = struct{}{}
	}
	for h := range recv {
		allHosts[h] = struct{}{}
	}
	type hostEntry struct {
		name  string
		total int
	}
	hosts := make([]hostEntry, 0, len(allHosts))
	for h := range allHosts {
		hosts = append(hosts, hostEntry{h, sent[h] + recv[h]})
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].total > hosts[j].total })
	if len(hosts) > 10 {
		hosts = hosts[:10]
	}
	hostRows := make([][]string, len(hosts))
	for i, h := range hosts {
		hostRows[i] = []string{h.name, config.FmtNum(sent[h.name]), config.FmtNum(recv[h.name]), config.FmtNum(h.total)}
	}
	table(&b, []string{"Host", "Sent (packets)", "Received (packets)", "Total"}, hostRows)

	return b.String()
}

func heading(b *strings.Builder, level int, text string) {
	b.WriteString("\n")
	b.WriteString(strings.Repeat("#", level) + " " + text)
	b.WriteString("\n\n")
}

func table(b *strings.Builder, headers []string, rows [][]string) {
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("|" + strings.Join(seps, "|") + "|\n")
	for _, row := range rows {
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	b.WriteString("\n")
}

// htmlTemplate is the self-contained HTML page template.
// Arguments (in order): pcap title, filename, generated, duration, packets, bytes, grpcCalls,
// filterBadge, flowDiagram, trafficSeqSection, grpcIdx (zoomIn param), seqDiagram,
// grpcIdx (toolbar zoomIn), grpcIdx (toolbar zoomOut), grpcIdx (toolbar resetZoom).
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Temporal Analysis — %s</title>
  <script src="https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/svg-pan-zoom@3.6.1/dist/svg-pan-zoom.min.js"></script>
  <style>
    * { box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, sans-serif;
      max-width: 1600px; margin: 0 auto; padding: 2rem 3rem;
      background: #f4f6fb; color: #1a1a2e;
    }
    h1 { border-bottom: 3px solid #6c63ff; padding-bottom: .5rem; margin-bottom: .75rem; }
    h2 { color: #4a4a8a; margin-top: 2.5rem; margin-bottom: .25rem; }
    .meta {
      display: flex; flex-wrap: wrap; gap: .75rem;
      color: #555; font-size: .9rem; margin-bottom: 2rem;
    }
    .meta span {
      background: white; padding: .3rem .8rem;
      border-radius: 4px; box-shadow: 0 1px 3px rgba(0,0,0,.1);
    }
    .hint { color: #888; font-size: .85rem; margin: .25rem 0 .75rem; }
    .card {
      background: white; border-radius: 10px;
      box-shadow: 0 2px 10px rgba(0,0,0,.08); margin: .5rem 0 1.5rem;
      display: flex; flex-direction: column;
    }
    .card-toolbar {
      display: flex; align-items: center; gap: .5rem;
      padding: .75rem 1.25rem; border-bottom: 1px solid #f0f0f0;
    }
    .card-toolbar button {
      padding: .25rem .7rem; border: 1px solid #ddd; border-radius: 4px;
      background: #f8f9fa; cursor: pointer; font-size: .8rem; color: #444;
    }
    .card-toolbar button:hover { background: #e9ecef; }
    .card-toolbar .zoom-hint {
      margin-left: auto; color: #aaa; font-size: .78rem;
    }
    .diagram-wrap {
      height: 68vh; min-height: 420px; overflow: hidden;
      border-radius: 0 0 10px 10px;
    }
    .diagram-wrap .mermaid { width: 100%%; height: 100%%; }
    .diagram-wrap .mermaid svg { width: 100%% !important; height: 100%% !important; }
    footer {
      margin-top: 3rem; color: #bbb; font-size: .8rem; text-align: center;
      border-top: 1px solid #e0e0e0; padding-top: 1rem;
    }
  </style>
</head>
<body>

  <h1>Temporal Traffic Analysis</h1>
  <div class="meta">
    <span><strong>File:</strong> %s</span>
    <span><strong>Generated:</strong> %s</span>
    <span><strong>Duration:</strong> %s</span>
    <span><strong>Packets:</strong> %s</span>
    <span><strong>Bytes:</strong> %s</span>
    <span><strong>gRPC Calls:</strong> %s</span>
    %s
  </div>

  <h2>Data Flow Diagram</h2>
  <p class="hint">Each arrow shows protocol, total packet count, and bytes transferred in that direction.</p>
  <div class="card">
    <div class="card-toolbar">
      <button onclick="zoomIn(0)">&#xFF0B; Zoom in</button>
      <button onclick="zoomOut(0)">&#xFF0D; Zoom out</button>
      <button onclick="resetZoom(0)">&#x27F3; Reset</button>
      <span class="zoom-hint">Scroll to zoom &nbsp;&middot;&nbsp; Drag to pan</span>
    </div>
    <div class="diagram-wrap">
      <div class="mermaid">
%s
      </div>
    </div>
  </div>
%s
  <h2>gRPC Sequence Diagram</h2>
  <p class="hint">Consecutive identical calls from the same source are compressed (xN). Sequence numbers show call order.</p>
  <div class="card">
    <div class="card-toolbar">
      <button onclick="zoomIn(%d)">&#xFF0B; Zoom in</button>
      <button onclick="zoomOut(%d)">&#xFF0D; Zoom out</button>
      <button onclick="resetZoom(%d)">&#x27F3; Reset</button>
      <span class="zoom-hint">Scroll to zoom &nbsp;&middot;&nbsp; Drag to pan</span>
    </div>
    <div class="diagram-wrap">
      <div class="mermaid">
%s
      </div>
    </div>
  </div>

  <footer>Generated by temporal-analyze &mdash; temporalcoms</footer>

  <script>
    const panZoomInstances = [];

    mermaid.initialize({
      startOnLoad: false,
      theme: "default",
      sequence: { showSequenceNumbers: true, mirrorActors: false, useMaxWidth: false },
      flowchart: { curve: "basis", useMaxWidth: false }
    });

    window.addEventListener("load", async function () {
      await mermaid.run({ querySelector: ".mermaid" });

      document.querySelectorAll(".diagram-wrap .mermaid svg").forEach(function (svg) {
        svg.style.width = "100%%";
        svg.style.height = "100%%";
        var instance = svgPanZoom(svg, {
          zoomEnabled: true,
          controlIconsEnabled: false,
          fit: true,
          center: true,
          minZoom: 0.05,
          maxZoom: 30,
          zoomScaleSensitivity: 0.3,
          mouseWheelZoomEnabled: true
        });
        panZoomInstances.push(instance);
      });
    });

    function zoomIn(i)    { if (panZoomInstances[i]) panZoomInstances[i].zoomIn(); }
    function zoomOut(i)   { if (panZoomInstances[i]) panZoomInstances[i].zoomOut(); }
    function resetZoom(i) {
      if (panZoomInstances[i]) {
        panZoomInstances[i].resetZoom();
        panZoomInstances[i].fit();
        panZoomInstances[i].center();
      }
    }
  </script>
</body>
</html>
`
