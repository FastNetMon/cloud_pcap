package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func compressAndUpload(cfg *Config, pcapPath string) error {
	pcapPath = filepath.Clean(pcapPath)

	log.Printf("Processing %s", pcapPath)

	originalInfo, err := os.Stat(pcapPath)
	if err != nil {
		return fmt.Errorf("pcap file not found: %w", err)
	}

	// Compress with bzip2 (replaces original file with .bz2)
	log.Printf("Compressing %s", pcapPath)
	bzipCmd := exec.Command("bzip2", pcapPath)
	if out, err := bzipCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bzip2 failed: %s: %w", string(out), err)
	}
	compressedPath := pcapPath + ".bz2"
	compressedInfo, err := os.Stat(compressedPath)
	if err != nil {
		return fmt.Errorf("stat compressed file: %w", err)
	}
	log.Printf("Compression complete: %s", formatCompressionStats(filepath.Base(pcapPath), filepath.Base(compressedPath), originalInfo.Size(), compressedInfo.Size()))

	// Upload to S3 with year/month/day folder structure
	now := time.Now().UTC()
	key := fmt.Sprintf("%s%d/%02d/%02d/%s",
		cfg.S3.Prefix, now.Year(), now.Month(), now.Day(),
		filepath.Base(compressedPath))
	log.Printf("Uploading to s3://%s/%s", cfg.S3.Bucket, key)
	if err := uploadToS3(cfg, compressedPath, key); err != nil {
		return fmt.Errorf("upload failed (file kept at %s): %w", compressedPath, err)
	}

	log.Printf("Upload complete: %s", filepath.Base(compressedPath))

	if cfg.DeleteAfterUpload {
		if err := os.Remove(compressedPath); err != nil {
			log.Printf("WARNING: failed to delete %s: %v", compressedPath, err)
		}
	}

	return nil
}

func formatCompressionStats(originalName, compressedName string, originalSize, compressedSize int64) string {
	if compressedSize <= 0 {
		return fmt.Sprintf("%s -> %s (%d bytes -> %d bytes, ratio unavailable)", originalName, compressedName, originalSize, compressedSize)
	}

	return fmt.Sprintf("%s -> %s (%d bytes -> %d bytes, ratio %.2f:1)", originalName, compressedName, originalSize, compressedSize, float64(originalSize)/float64(compressedSize))
}

func uploadToS3(cfg *Config, filePath, key string) error {
	ctx := context.Background()

	sdkCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.S3.AccessKeyID,
				cfg.S3.SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}

	client := s3.NewFromConfig(sdkCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + cfg.S3.Endpoint)
		o.UsePathStyle = true
	})

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	tm := transfermanager.New(client)
	_, err = tm.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(cfg.S3.Bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("s3 upload: %w", err)
	}

	return nil
}
