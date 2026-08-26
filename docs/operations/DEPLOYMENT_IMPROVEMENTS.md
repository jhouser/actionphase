# Production Deployment Improvements

## Issues Addressed

### 1. ✅ Monitoring Scripts Not Running
**Problem**: `check-disk.sh` and `check-ssl.sh` scripts exist but weren't scheduled in cron.

**Solution**: Updated `terraform/user-data.sh` to add monitoring cron jobs:

```bash
# Daily disk space monitoring at 6 AM UTC
0 6 * * * ubuntu cd /opt/actionphase && ./scripts/check-disk.sh --threshold 70

# Weekly SSL certificate check at 7 AM UTC on Mondays
0 7 * * 1 ubuntu cd /opt/actionphase && ./scripts/check-ssl.sh

# Clean old Docker resources weekly at 4 AM UTC on Sundays
0 4 * * 0 ubuntu docker system prune -f
```

**Files Modified**:
- `terraform/user-data.sh` (lines 146-163)

---

### 2. ✅ Container Logs Not Persisted
**Problem**: Logs stored in `/var/lib/docker/containers/` are lost when containers are removed.

**Solution**: Created comprehensive logging strategy with three options:

#### Option 1: AWS CloudWatch Logs (Best for AWS)
- Fully managed, searchable logs
- CloudWatch Alarms integration
- Cost: ~$5-10/month
- See: `docs/operations/LOGGING_STRATEGY.md` (lines 36-122)

#### Option 2: Local Volume Persistence (Best for Self-Hosted) ✅ **Recommended**
- Zero cost
- Logs persist on host filesystem
- 30-day retention with rotation
- Created: `docker-compose.logging.yml`
- See: `docs/operations/LOGGING_STRATEGY.md` (lines 126-216)

#### Option 3: Hybrid Approach
- Critical logs → CloudWatch
- Debug logs → Local volumes
- See: `docs/operations/LOGGING_STRATEGY.md` (lines 220-238)

**Files Created**:
- `docker-compose.logging.yml` - Log persistence configuration
- `docs/operations/LOGGING_STRATEGY.md` - Comprehensive guide
- `docs/operations/LOGGING_QUICK_REFERENCE.md` - Quick reference

**Files Modified**:
- `terraform/user-data.sh`:
  - Added log directory creation (line 119)
  - Enhanced logrotate config (lines 173-200)

---

## What Changed

### terraform/user-data.sh

1. **Added Monitoring Cron Jobs** (lines 155-162):
   - Daily disk monitoring (6 AM UTC)
   - Weekly SSL check (Mondays 7 AM UTC)
   - Weekly Docker cleanup (Sundays 4 AM UTC)

2. **Created Log Directories** (line 119):
   ```bash
   mkdir -p /opt/actionphase/logs/{backend,frontend,nginx,postgres,backup}
   ```

3. **Enhanced Log Rotation** (lines 173-200):
   - 30-day retention (up from 7 days)
   - Separate configs for app vs backup logs
   - Automatic nginx log file rotation

### New Files

1. **docker-compose.logging.yml**
   - Drop-in override for log persistence
   - Maps container logs to host volumes
   - Configured for all services

2. **docs/operations/LOGGING_STRATEGY.md**
   - Three logging architecture options
   - Cost-benefit analysis
   - Implementation instructions
   - Monitoring setup
   - Best practices

3. **docs/operations/LOGGING_QUICK_REFERENCE.md**
   - Quick commands for log viewing
   - Search examples (grep, jq)
   - Troubleshooting guide
   - Emergency procedures

---

## How to Deploy

### Current Deployment (No Change Required)
```bash
# This still works as before
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```
⚠️ **Logs are NOT persisted**

### With Log Persistence (Recommended)
```bash
# 1. Create log directories (done automatically by user-data.sh)
mkdir -p /opt/actionphase/logs/{backend,frontend,nginx,postgres,backup}
chown -R ubuntu:ubuntu /opt/actionphase/logs

# 2. Deploy with logging enabled
docker-compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  -f docker-compose.logging.yml \
  up -d
```
✅ **Logs persist even when containers are removed**

---

## Monitoring Schedule

| Task | Frequency | Time (UTC) | Purpose |
|------|-----------|------------|---------|
| Database Backup | Daily | 2:00 AM | Data protection |
| AMI Snapshot | Weekly (Sunday) | 3:00 AM | Server recovery |
| Docker Cleanup | Weekly (Sunday) | 4:00 AM | Disk space |
| Disk Monitoring | Daily | 6:00 AM | Prevent disk full |
| SSL Check | Weekly (Monday) | 7:00 AM | Certificate expiry |

All cron jobs log to `/opt/actionphase/logs/` or `/var/log/`.

---

## Log Retention

### Application Logs
- **Retention**: 14 days (`rotate 14` in `terraform/user-data.sh`)
- **Rotation**: Daily, `copytruncate` (Go holds the file open)
- **Compression**: Yes, `delaycompress` (after 1 day)
- **Location**: `/opt/actionphase/logs/{backend,nginx,frontend}/`

### Docker Container Logs (json-file)

Per `docker-compose.logging.yml`:

| Service | max-size | max-file |
|---|---|---|
| backend | 100m | 10 |
| nginx | 100m | 10 |
| frontend | 50m | 5 |
| db | 20m | 5 |

### Backup Logs
- **Retention**: 4 weeks
- **Rotation**: Weekly
- **Location**: `/opt/actionphase/backups/`

---

## Quick Commands

### View Logs
```bash
# Live logs
docker-compose logs -f backend

# Persisted logs (with logging.yml)
tail -f /opt/actionphase/logs/backend/app.log

# Search for errors
grep '"level":"error"' logs/backend/app.log | jq .
```

### Check Monitoring
```bash
# Run disk check manually
/opt/actionphase/scripts/check-disk.sh

# Run SSL check manually
/opt/actionphase/scripts/check-ssl.sh

# View monitoring logs
tail -f /opt/actionphase/logs/disk-monitor.log
tail -f /opt/actionphase/logs/ssl-monitor.log
```

### Disk Management
```bash
# Check log disk usage
du -sh /opt/actionphase/logs/*

# Rotate logs immediately
logrotate -f /etc/logrotate.d/actionphase

# Clean old Docker resources
docker system prune -a
```

---

## Testing

### Verify Cron Jobs Installed
```bash
# Check cron configuration
cat /etc/cron.d/actionphase

# View cron logs
grep CRON /var/log/syslog | tail -20
```

### Verify Log Persistence
```bash
# 1. Start services with logging
docker-compose -f docker-compose.yml -f docker-compose.prod.yml -f docker-compose.logging.yml up -d

# 2. Generate some logs
curl http://localhost:3000/health

# 3. Check logs are being written
ls -lh /opt/actionphase/logs/backend/
tail /opt/actionphase/logs/backend/app.log

# 4. Remove container
docker-compose down

# 5. Verify logs still exist
ls -lh /opt/actionphase/logs/backend/
# Logs should still be there!
```

### Verify Logrotate
```bash
# Test rotation (dry run)
logrotate -d /etc/logrotate.d/actionphase

# Force rotation
logrotate -f /etc/logrotate.d/actionphase

# Check rotated files
ls -lh /opt/actionphase/logs/backend/
```

---

## Migration Guide

### For Existing Deployments

1. **Update user-data.sh** (if redeploying EC2):
   - Already done in `terraform/user-data.sh`
   - Monitoring cron jobs will be added automatically

2. **Enable log persistence** (optional but recommended):
   ```bash
   # SSH to server
   ssh ubuntu@your-server.com

   # Create log directories
   mkdir -p /opt/actionphase/logs/{backend,frontend,nginx,postgres,backup}
   chown -R ubuntu:ubuntu /opt/actionphase/logs

   # Download docker-compose.logging.yml
   cd /opt/actionphase
   # (copy docker-compose.logging.yml to server)

   # Redeploy with logging
   docker-compose \
     -f docker-compose.yml \
     -f docker-compose.prod.yml \
     -f docker-compose.logging.yml \
     up -d
   ```

3. **Add monitoring cron jobs** (if not redeploying):
   ```bash
   # Copy new cron config
   sudo tee /etc/cron.d/actionphase << 'EOF'
   # Daily database backup at 2 AM UTC
   0 2 * * * ubuntu cd /opt/actionphase && ./scripts/backup-to-s3.sh >> /opt/actionphase/backups/backup.log 2>&1

   # Weekly AMI snapshot at 3 AM UTC on Sundays
   0 3 * * 0 root /usr/local/bin/aws ec2 create-image --instance-id $(ec2-metadata --instance-id | cut -d " " -f 2) --name "actionphase-ami-$(date +\%Y\%m\%d)" --no-reboot >> /var/log/ami-snapshot.log 2>&1

   # Daily disk space monitoring at 6 AM UTC
   0 6 * * * ubuntu cd /opt/actionphase && ./scripts/check-disk.sh --threshold 70 >> /opt/actionphase/logs/disk-monitor.log 2>&1

   # Weekly SSL certificate check at 7 AM UTC on Mondays
   0 7 * * 1 ubuntu cd /opt/actionphase && ./scripts/check-ssl.sh >> /opt/actionphase/logs/ssl-monitor.log 2>&1

   # Clean old Docker resources weekly at 4 AM UTC on Sundays
   0 4 * * 0 ubuntu docker system prune -f >> /var/log/docker-cleanup.log 2>&1
   EOF

   # Reload cron
   sudo systemctl reload cron
   ```

---

## Next Steps

### Immediate Actions
- [x] Update user-data.sh with monitoring cron jobs
- [x] Create log persistence configuration
- [x] Document logging strategies
- [ ] **Test in staging environment**
- [ ] **Choose logging strategy** (Option 2 recommended to start)
- [ ] Deploy to production

### Optional Enhancements
- [ ] Set up Slack webhooks for alerts
- [ ] Configure CloudWatch Logs (if using AWS)
- [ ] Create log analysis dashboard
- [ ] Set up automated log backups to S3

### Monitoring
- [ ] Monitor disk usage after enabling log persistence
- [ ] Verify cron jobs are running
- [ ] Review monitoring logs weekly
- [ ] Adjust retention periods if needed

---

## Documentation References

- **Logging Strategy**: `docs/operations/LOGGING_STRATEGY.md`
- **Quick Reference**: `docs/operations/LOGGING_QUICK_REFERENCE.md`
- **Deployment**: `terraform/user-data.sh`
- **Monitoring Scripts**:
  - `scripts/check-disk.sh`
  - `scripts/check-ssl.sh`

---

## Summary

✅ **Monitoring**: Added cron jobs for disk and SSL monitoring
✅ **Log Persistence**: Created drop-in solution with `docker-compose.logging.yml`
✅ **Documentation**: Comprehensive guides for implementation and operations
✅ **Zero Cost**: Recommended solution (Option 2) has no additional costs
✅ **Backward Compatible**: Existing deployments continue to work unchanged

**Recommendation**: Deploy with `docker-compose.logging.yml` for production to ensure logs are persisted and searchable for troubleshooting.
