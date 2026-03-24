FROM alpine:3.19
RUN apk --no-cache add tshark
# Ring-buffer: 5 files × 50 MB = up to 250 MB of rolling captures.
# Files land at /captures/temporal_NNNNN_TIMESTAMP.pcap
CMD ["tshark", "-i", "temporal-net", "-w", "/captures/temporal.pcap", "-b", "filesize:51200", "-b", "files:5"]
