package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

func runCapture(cfg *Config) error {
	if _, err := exec.LookPath("tcpdump"); err != nil {
		return fmt.Errorf("tcpdump not found in PATH: %w", err)
	}
	if _, err := exec.LookPath("bzip2"); err != nil {
		return fmt.Errorf("bzip2 not found in PATH: %w", err)
	}

	if err := os.MkdirAll(cfg.CaptureDir, 0750); err != nil {
		return fmt.Errorf("create capture directory: %w", err)
	}

	maxBytes := int64(cfg.MaxFileSizeGB * 1024 * 1024 * 1024)

	ctx := &captureContext{
		cfg:      cfg,
		maxBytes: maxBytes,
		stopCh:   make(chan struct{}),
	}

	var wg sync.WaitGroup
	for _, iface := range cfg.Interfaces {
		wg.Add(1)
		go func(iface string) {
			defer wg.Done()
			ctx.captureLoop(iface)
		}(iface)
	}

	log.Printf("Capturing on %d interface(s). Press Ctrl+C to stop.", len(cfg.Interfaces))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received %v, shutting down...", sig)

	close(ctx.stopCh)
	ctx.killAll()
	wg.Wait()

	// Wait for any in-flight compress+upload goroutines
	ctx.uploadWg.Wait()
	log.Println("All captures stopped.")
	return nil
}

type captureContext struct {
	cfg      *Config
	maxBytes int64
	stopCh   chan struct{}

	mu       sync.Mutex
	procs    []*os.Process
	uploadWg sync.WaitGroup
}

func (c *captureContext) trackProcess(p *os.Process) {
	c.mu.Lock()
	c.procs = append(c.procs, p)
	c.mu.Unlock()
}

func (c *captureContext) untrackProcess(p *os.Process) {
	c.mu.Lock()
	for i, proc := range c.procs {
		if proc == p {
			c.procs = append(c.procs[:i], c.procs[i+1:]...)
			break
		}
	}
	c.mu.Unlock()
}

func (c *captureContext) killAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.procs {
		p.Signal(syscall.SIGTERM)
	}
}

func (c *captureContext) captureLoop(iface string) {
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		pcapFile := filepath.Join(
			c.cfg.CaptureDir,
			fmt.Sprintf("%s-%s.pcap", iface, time.Now().UTC().Format("2006-01-02-15-04-05")),
		)

		cmd := buildTcpdumpCmd(c.cfg, iface, pcapFile)
		log.Printf("[%s] Starting capture -> %s", iface, filepath.Base(pcapFile))

		if err := cmd.Start(); err != nil {
			log.Printf("[%s] ERROR: Failed to start tcpdump: %v", iface, err)
			select {
			case <-c.stopCh:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		c.trackProcess(cmd.Process)

		// Wait for file to reach max size or stop signal
		reason := c.waitForRotation(pcapFile)

		// Stop this tcpdump instance
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
		c.untrackProcess(cmd.Process)

		if reason == "stop" {
			c.processFile(iface, pcapFile)
			return
		}

		log.Printf("[%s] File reached %.2f GB, rotating", iface, c.cfg.MaxFileSizeGB)
		c.processFile(iface, pcapFile)
	}
}

// waitForRotation polls the pcap file size until it exceeds maxBytes or stop is signaled.
// Returns "size" or "stop".
func (c *captureContext) waitForRotation(pcapFile string) string {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return "stop"
		case <-ticker.C:
			info, err := os.Stat(pcapFile)
			if err != nil {
				continue // file may not exist yet
			}
			if info.Size() >= c.maxBytes {
				return "size"
			}
		}
	}
}

// processFile launches async compress+upload for a completed pcap file.
func (c *captureContext) processFile(iface, pcapFile string) {
	info, err := os.Stat(pcapFile)
	if err != nil || info.Size() == 0 {
		os.Remove(pcapFile)
		return
	}

	c.uploadWg.Add(1)
	go func() {
		defer c.uploadWg.Done()
		if err := compressAndUpload(c.cfg, pcapFile); err != nil {
			log.Printf("[%s] ERROR: compress/upload failed for %s: %v", iface, filepath.Base(pcapFile), err)
		}
	}()
}

func buildTcpdumpCmd(cfg *Config, iface, outputPath string) *exec.Cmd {
	args := []string{
		"-n",
		"-i", iface,
		"-w", outputPath,
	}

	if cfg.SnapLen > 0 {
		args = append(args, "-s", fmt.Sprintf("%d", cfg.SnapLen))
	}

	if cfg.BPFFilter != "" {
		args = append(args, cfg.BPFFilter)
	}

	cmd := exec.Command("tcpdump", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
