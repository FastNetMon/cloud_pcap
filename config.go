package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type S3Config struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

type Config struct {
	Interfaces        []string `json:"interfaces"`
	CaptureDir        string   `json:"capture_dir"`
	MaxFileSizeGB     float64  `json:"max_file_size_gb"`
	BPFFilter         string   `json:"bpf_filter"`
	SnapLen           int      `json:"snap_len"`
	S3                S3Config `json:"s3"`
	DeleteAfterUpload bool     `json:"delete_after_upload"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if len(cfg.Interfaces) == 0 {
		return nil, fmt.Errorf("no interfaces configured")
	}
	if cfg.CaptureDir == "" {
		cfg.CaptureDir = "/pcaps"
	}
	if cfg.MaxFileSizeGB <= 0 {
		cfg.MaxFileSizeGB = 1
	}
	if cfg.S3.Endpoint == "" {
		return nil, fmt.Errorf("s3.endpoint is required")
	}
	if cfg.S3.Bucket == "" {
		return nil, fmt.Errorf("s3.bucket is required")
	}
	if cfg.S3.AccessKeyID == "" || cfg.S3.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3 credentials are required")
	}
	if cfg.S3.Region == "" {
		cfg.S3.Region = "us-east-1"
	}

	return &cfg, nil
}
