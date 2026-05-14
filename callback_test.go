package main

import "testing"

func TestFormatCompressionStats(t *testing.T) {
	t.Run("includes compression ratio", func(t *testing.T) {
		got := formatCompressionStats("capture.pcap", "capture.pcap.bz2", 1024, 256)
		want := "capture.pcap -> capture.pcap.bz2 (1024 bytes -> 256 bytes, ratio 4.00:1)"
		if got != want {
			t.Fatalf("formatCompressionStats() = %q, want %q", got, want)
		}
	})

	t.Run("handles zero compressed size", func(t *testing.T) {
		got := formatCompressionStats("capture.pcap", "capture.pcap.bz2", 1024, 0)
		want := "capture.pcap -> capture.pcap.bz2 (1024 bytes -> 0 bytes, ratio unavailable)"
		if got != want {
			t.Fatalf("formatCompressionStats() = %q, want %q", got, want)
		}
	})

	t.Run("handles zero original size", func(t *testing.T) {
		got := formatCompressionStats("capture.pcap", "capture.pcap.bz2", 0, 128)
		want := "capture.pcap -> capture.pcap.bz2 (0 bytes -> 128 bytes, ratio unavailable)"
		if got != want {
			t.Fatalf("formatCompressionStats() = %q, want %q", got, want)
		}
	})
}
