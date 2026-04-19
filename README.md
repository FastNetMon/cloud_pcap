# cloud_pcap

Network packet capture tool that runs `tcpdump` on configured interfaces, compresses rotated capture files with `bzip2`, and uploads them to S3-compatible storage (Backblaze B2).

## Requirements

- Go 1.22+
- `tcpdump`
- `bzip2`
- Root privileges (for packet capture)

## Build

```bash
go build -o cloud_pcap .
```

## Configuration

Copy `config.example.json` to `config.json` and edit:

```json
{
    "interfaces": ["ens256", "ens224"],
    "capture_dir": "/pcaps",
    "max_file_size_mb": 1024,
    "bpf_filter": "",
    "snap_len": 0,
    "s3": {
        "endpoint": "s3.eu-central-003.backblazeb2.com",
        "region": "eu-central-003",
        "bucket": "company-name-traffic",
        "prefix": "traffic/",
        "access_key_id": "YOUR_KEY",
        "secret_access_key": "YOUR_SECRET"
    },
    "delete_after_upload": true
}
```

### Fields

| Field | Description | Default |
|---|---|---|
| `interfaces` | Network interfaces to capture | *(required)* |
| `capture_dir` | Directory for pcap files | `/pcaps` |
| `max_file_size_mb` | Rotate when pcap file reaches this size in MB | `1024` |
| `bpf_filter` | BPF filter expression (e.g. `"tcp port 80"`) | *(none)* |
| `snap_len` | Snapshot length in bytes (0 = default 262144) | `0` |
| `s3.endpoint` | S3-compatible endpoint host | *(required)* |
| `s3.region` | S3 region | `us-east-1` |
| `s3.bucket` | S3 bucket name | *(required)* |
| `s3.prefix` | Key prefix for uploaded files | *(none)* |
| `s3.access_key_id` | S3 access key | *(required)* |
| `s3.secret_access_key` | S3 secret key | *(required)* |
| `delete_after_upload` | Delete compressed file after successful upload | `false` |

## Usage

```bash
# Start capture (requires root for tcpdump)
sudo ./cloud_pcap --config config.json
```

The tool will:
1. Start `tcpdump` on each configured interface
2. Monitor capture file size, rotate when it reaches `max_file_size_mb`
3. Compress completed files with `bzip2`
4. Upload `.pcap.bz2` files to S3
5. Optionally delete local files after successful upload

### How it works

- For each interface, a goroutine runs a capture loop: start tcpdump, poll file size, stop tcpdump when the limit is reached, then compress+upload in the background and immediately start a new tcpdump
- Compression and upload run asynchronously so capture is not interrupted
- On SIGINT/SIGTERM, all tcpdump processes are stopped and pending uploads finish before exit

## Install as systemd service

```bash
sudo cp cloud_pcap /usr/local/bin/
sudo mkdir -p /etc/cloud_pcap
sudo cp config.json /etc/cloud_pcap/config.json
```

Create `/etc/systemd/system/cloud_pcap.service`:

```ini
[Unit]
Description=Cloud PCAP Capture
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cloud_pcap --config /etc/cloud_pcap/config.json
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cloud_pcap
```
