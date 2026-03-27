package filter_test

import (
	"testing"
	"temporal-analyze/internal/filter"
	"temporal-analyze/internal/tshark"
)

// makePacket creates a test packet with the given src, dst, dport, sport, proto.
func makePacket(src, dst, dport, sport, proto string) tshark.Packet {
	return tshark.Packet{Src: src, Dst: dst, Dport: dport, Sport: sport, Proto: proto}
}

// makeCall creates a test gRPC call.
func makeCall(src, dst, method string) tshark.GRPCCall {
	return tshark.GRPCCall{Src: src, Dst: dst, Method: method}
}

var testPackets = []tshark.Packet{
	makePacket("172.20.0.41", "172.20.0.21", "7233", "54321", "HTTP2"),  // worker → frontend gRPC
	makePacket("172.20.0.21", "172.20.0.22", "7234", "54322", "HTTP2"),  // frontend → history gRPC
	makePacket("172.20.0.22", "172.20.0.10", "5432", "54323", "PGSQL"),  // history → postgres
	makePacket("172.20.0.30", "172.20.0.21", "7233", "54324", "HTTP2"),  // ui → frontend gRPC
	makePacket("172.20.0.41", "172.20.0.21", "7233", "54325", "TCP"),    // raw TCP on gRPC port
}

var testCalls = []tshark.GRPCCall{
	makeCall("hello-world-starter", "temporal-frontend", "StartWorkflowExecution"),
	makeCall("temporal-frontend", "temporal-history", "RecordWorkflowTaskStarted"),
	makeCall("temporal-ui", "temporal-frontend", "GetWorkflowExecutionHistory"),
}

func TestApply_NoFilter(t *testing.T) {
	pkts, calls := filter.Apply(testPackets, testCalls, filter.FilterOptions{})
	if len(pkts) != len(testPackets) {
		t.Errorf("expected %d packets, got %d", len(testPackets), len(pkts))
	}
	if len(calls) != len(testCalls) {
		t.Errorf("expected %d calls, got %d", len(testCalls), len(calls))
	}
}

func TestApply_OnlyGRPC(t *testing.T) {
	pkts, calls := filter.Apply(testPackets, testCalls, filter.FilterOptions{
		Only: []string{"grpc"},
	})
	for _, p := range pkts {
		if p.Dport != "7233" && p.Dport != "7234" && p.Dport != "7235" && p.Dport != "7239" &&
			p.Sport != "7233" && p.Sport != "7234" && p.Sport != "7235" && p.Sport != "7239" {
			t.Errorf("non-gRPC packet passed through: %+v", p)
		}
	}
	if len(calls) != len(testCalls) {
		t.Errorf("gRPC calls should be preserved when --only grpc, got %d", len(calls))
	}
}

func TestApply_ExcludePgsql(t *testing.T) {
	pkts, _ := filter.Apply(testPackets, testCalls, filter.FilterOptions{
		Exclude: []string{"pgsql"},
	})
	for _, p := range pkts {
		if p.Dport == "5432" || p.Sport == "5432" {
			t.Error("pgsql packet should be excluded")
		}
	}
}

func TestApply_ExcludeGRPC_ClearsCallList(t *testing.T) {
	_, calls := filter.Apply(testPackets, testCalls, filter.FilterOptions{
		Exclude: []string{"grpc"},
	})
	if len(calls) != 0 {
		t.Errorf("calls should be cleared when grpc excluded, got %d", len(calls))
	}
}

func TestApply_OnlyHost(t *testing.T) {
	pkts, calls := filter.Apply(testPackets, testCalls, filter.FilterOptions{
		OnlyHosts: []string{"hello-world-starter"},
	})
	// Only packets where src or dst is hello-world-starter (172.20.0.41)
	for _, p := range pkts {
		if p.Src != "172.20.0.41" && p.Dst != "172.20.0.41" {
			t.Errorf("packet from/to unexpected host: %+v", p)
		}
	}
	for _, c := range calls {
		if c.Src != "hello-world-starter" && c.Dst != "hello-world-starter" {
			t.Errorf("call from/to unexpected host: %+v", c)
		}
	}
}

func TestApply_ExcludeHost(t *testing.T) {
	pkts, calls := filter.Apply(testPackets, testCalls, filter.FilterOptions{
		ExcludeHosts: []string{"temporal-history"},
	})
	for _, p := range pkts {
		if p.Src == "172.20.0.22" || p.Dst == "172.20.0.22" {
			t.Errorf("temporal-history packet should be excluded: %+v", p)
		}
	}
	for _, c := range calls {
		if c.Src == "temporal-history" || c.Dst == "temporal-history" {
			t.Errorf("temporal-history call should be excluded: %+v", c)
		}
	}
}

func TestApply_NoInterservice(t *testing.T) {
	pkts, calls := filter.Apply(testPackets, testCalls, filter.FilterOptions{
		NoInterservice: true,
	})
	interservice := map[string]string{
		"172.20.0.22": "temporal-history",
		"172.20.0.23": "temporal-matching",
		"172.20.0.24": "temporal-internal-worker",
	}
	for _, p := range pkts {
		if _, bad := interservice[p.Src]; bad {
			t.Errorf("interservice src should be excluded: %+v", p)
		}
		if _, bad := interservice[p.Dst]; bad {
			t.Errorf("interservice dst should be excluded: %+v", p)
		}
	}
	for _, c := range calls {
		if c.Src == "temporal-history" || c.Dst == "temporal-history" {
			t.Errorf("interservice call should be excluded: %+v", c)
		}
	}
}

func TestShowTrafficSeq(t *testing.T) {
	cases := []struct {
		opts     filter.FilterOptions
		expected bool
	}{
		{filter.FilterOptions{}, true},
		{filter.FilterOptions{Only: []string{"grpc"}}, false},
		{filter.FilterOptions{Only: []string{"http2"}}, false},
		{filter.FilterOptions{Only: []string{"grpc", "http"}}, true},
		{filter.FilterOptions{Exclude: []string{"grpc"}}, true},
		{filter.FilterOptions{Only: []string{"pgsql"}}, true},
	}
	for _, tc := range cases {
		got := filter.ShowTrafficSeq(tc.opts)
		if got != tc.expected {
			t.Errorf("ShowTrafficSeq(%v) = %v, want %v", tc.opts, got, tc.expected)
		}
	}
}

func TestMatchesProtocol(t *testing.T) {
	cases := []struct {
		packet   tshark.Packet
		proto    string
		expected bool
	}{
		{makePacket("", "", "7233", "", "HTTP2"), "grpc", true},
		{makePacket("", "", "5432", "", "PGSQL"), "pgsql", true},
		{makePacket("", "", "5432", "", "PGSQL"), "postgresql", true},
		{makePacket("", "", "8080", "", "HTTP"), "http", true},
		{makePacket("", "", "1234", "", "ARP"), "arp", true},
		{makePacket("", "", "1234", "", "ARP"), "grpc", false},
		{makePacket("", "", "7233", "", "HTTP2"), "pgsql", false},
	}
	for _, tc := range cases {
		got := filter.MatchesProtocol(tc.packet, tc.proto)
		if got != tc.expected {
			t.Errorf("MatchesProtocol(%v, %q) = %v, want %v", tc.packet, tc.proto, got, tc.expected)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		opts     filter.FilterOptions
		expected string
	}{
		{filter.FilterOptions{}, ""},
		{filter.FilterOptions{Only: []string{"grpc"}}, "only: grpc"},
		{filter.FilterOptions{Exclude: []string{"pgsql"}}, "exclude: pgsql"},
		{filter.FilterOptions{OnlyHosts: []string{"temporal-frontend"}}, "host: temporal-frontend"},
		{filter.FilterOptions{NoInterservice: true}, "no-interservice"},
		{
			filter.FilterOptions{Only: []string{"grpc"}, NoInterservice: true},
			"only: grpc | no-interservice",
		},
	}
	for _, tc := range cases {
		got := filter.Describe(tc.opts)
		if got != tc.expected {
			t.Errorf("Describe(%v) = %q, want %q", tc.opts, got, tc.expected)
		}
	}
}
