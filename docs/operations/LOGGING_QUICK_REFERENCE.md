# Logging Quick Reference Guide

## Deploying with Log Persistence

**Log persistence is already the default.** `scripts/deploy-production.sh` sets:

```bash
COMPOSE_FILES="-f docker-compose.yml -f docker-compose.prod.yml -f docker-compose.logging.yml"
```

and falls back to the two-file form only if `docker-compose.logging.yml` is
missing. Normal deploys need no special flags:

```bash
./scripts/deploy-production.sh
```

For manual invocations, export the same variable once per shell so every
command targets the full stack:

```bash
export COMPOSE_FILES="-f docker-compose.yml -f docker-compose.prod.yml -f docker-compose.logging.yml"
docker-compose $COMPOSE_FILES up -d
```

> A bare `docker-compose <cmd>` uses only `docker-compose.yml` and will miss the
> prod and logging overrides entirely.

---

## Viewing Logs

### Live Container Logs (Real-time)
```bash
# All services
docker-compose $COMPOSE_FILES logs -f

# Specific service
docker-compose $COMPOSE_FILES logs -f backend

# Multiple services
docker-compose $COMPOSE_FILES logs -f backend nginx

# Last 100 lines
docker-compose $COMPOSE_FILES logs --tail 100 backend
```

### Persisted Application Logs (After enabling logging.yml)
```bash
# Backend structured JSON logs
tail -f logs/backend/app.log

# Nginx access logs
tail -f logs/nginx/access.log

# Nginx error logs
tail -f logs/nginx/error.log

# PostgreSQL logs
tail -f logs/postgres/postgresql.log
```

---

## Searching Logs

### Search Container Logs
```bash
# Search all backend logs for errors
docker-compose $COMPOSE_FILES logs backend | grep -i error

# Search for specific correlation ID
docker-compose $COMPOSE_FILES logs backend | grep "correlation_id=abc123"

# Search for user activity
docker-compose $COMPOSE_FILES logs backend | grep "user_id=42"
```

### Search Persisted Logs (JSON)
```bash
# Find all errors in last 1000 lines
tail -1000 logs/backend/app.log | jq 'select(.level == "error")'

# Find logs for specific user
grep '"user_id":123' logs/backend/app.log | jq .

# Find logs with specific correlation ID
grep '"correlation_id":"abc123"' logs/backend/app.log | jq .

# Extract error messages only
tail -1000 logs/backend/app.log | jq -r 'select(.level == "error") | .message'

# Find slow requests (>1000ms)
tail -1000 logs/backend/app.log | jq 'select(.duration_ms > 1000)'
```

### Search Nginx Access Logs
```bash
# Find all 500 errors
grep " 500 " logs/nginx/access.log

# Find requests from specific IP
grep "192.168.1.100" logs/nginx/access.log

# Count requests by status code
awk '{print $9}' logs/nginx/access.log | sort | uniq -c | sort -rn

# Find slowest requests
sort -t' ' -k10 -rn logs/nginx/access.log | head -20
```

---

## Log Rotation

### Manual Rotation
```bash
# Rotate logs immediately
logrotate -f /etc/logrotate.d/actionphase

# Test rotation without executing
logrotate -d /etc/logrotate.d/actionphase
```

### Check Rotation Status
```bash
# View rotation status
cat /var/lib/logrotate/status | grep actionphase

# Check rotated log files
ls -lh logs/backend/app.log*
```

---

## Disk Space Management

### Check Log Disk Usage
```bash
# Check total log directory size
du -sh logs/

# Check each service
du -sh logs/*

# Find largest log files
find logs/ -type f -exec du -h {} + | sort -rh | head -20

# Check old compressed logs
find logs/ -name "*.gz" -mtime +30
```

### Clean Old Logs
```bash
# Remove logs older than 30 days (be careful!)
find logs/ -name "*.gz" -mtime +30 -delete

# Remove logs older than 30 days (with confirmation)
find logs/ -name "*.gz" -mtime +30 -exec ls -lh {} \;
# Review, then:
find logs/ -name "*.gz" -mtime +30 -delete
```

---

## Troubleshooting Common Issues

### Container Won't Start
```bash
# Check container logs
docker logs actionphase-backend

# Check Docker daemon logs
journalctl -u docker -n 100

# Check for permission issues
ls -la /opt/actionphase/logs/
```

### Logs Not Appearing
```bash
# Check if logging volume is mounted
docker inspect actionphase-backend | grep -A 10 Mounts

# Verify log directory permissions
ls -la logs/backend/

# Check Docker logging driver
docker inspect actionphase-backend | grep LogConfig -A 10
```

### Disk Full
```bash
# Check disk usage
df -h

# Find what's using space
du -sh /var/lib/docker/containers/*
du -sh /opt/actionphase/logs/*

# Clean Docker resources
docker system prune -a

# Rotate logs immediately
logrotate -f /etc/logrotate.d/actionphase
```

---

## Monitoring Setup

### Daily Disk Check (Already in cron)
```bash
# Run manually
/opt/actionphase/scripts/check-disk.sh

# View disk monitor logs
tail -f /opt/actionphase/logs/disk-monitor.log
```

### Check Log Volume
```bash
# Create script to monitor log growth
cat > /opt/actionphase/scripts/check-log-volume.sh << 'EOF'
#!/bin/bash
LOG_SIZE=$(du -sm /opt/actionphase/logs | cut -f1)
if [ "$LOG_SIZE" -gt 5000 ]; then
    echo "[$(date)] WARNING: Logs directory is ${LOG_SIZE}MB"
    # Optional: Send alert
fi
EOF

chmod +x /opt/actionphase/scripts/check-log-volume.sh

# Add to cron
echo "0 */6 * * * ubuntu /opt/actionphase/scripts/check-log-volume.sh >> /var/log/log-volume.log 2>&1" | sudo tee -a /etc/cron.d/actionphase
```

---

## Export Logs for Analysis

### Export Last 24 Hours
```bash
# Backend logs
find logs/backend -name "app.log*" -mtime -1 -exec cat {} \; > backend-last-24h.json

# Nginx access logs
find logs/nginx -name "access.log*" -mtime -1 -exec cat {} \; > nginx-last-24h.log
```

### Export Specific Time Range
```bash
# Get logs between two timestamps (if logs have timestamps)
jq -r 'select(.timestamp >= "2025-01-15T00:00:00" and .timestamp <= "2025-01-16T00:00:00")' \
  logs/backend/app.log > logs-jan-15.json
```

### Compress for Shipping
```bash
# Compress logs for sending to support
tar -czf actionphase-logs-$(date +%Y%m%d).tar.gz logs/

# Upload to S3 (if configured)
aws s3 cp actionphase-logs-$(date +%Y%m%d).tar.gz \
  s3://actionphase-debug-logs/
```

---

## Performance Impact

### Current Logging Overhead

| Logging Method | CPU Impact | Disk I/O | Notes |
|---------------|-----------|----------|-------|
| Docker json-file | ~1-2% | Low | Default, minimal overhead |
| Volume persistence | ~2-3% | Medium | Additional disk writes |
| Both | ~3-5% | Medium | Dual logging |

### Optimization Tips

1. **Reduce log verbosity in production**:
```env
LOG_LEVEL=info  # Don't use debug in production
```

2. **Use async logging** (already implemented in backend via buffered logger)

3. **Compress old logs**:
```bash
# Compress logs older than 7 days
find logs/ -name "*.log" -mtime +7 -exec gzip {} \;
```

4. **Monitor log volume**:
```bash
# Alert if logs grow too fast
watch -n 300 'du -sh logs/'
```

---

## Emergency Procedures

### Out of Disk Space
```bash
# 1. Check what's using space
df -h
du -sh /var/lib/docker/containers/*
du -sh /opt/actionphase/logs/*

# 2. Emergency cleanup
docker system prune -a -f
find /opt/actionphase/logs -name "*.gz" -mtime +7 -delete
find /opt/actionphase/logs -name "*.log" -mtime +3 -exec gzip {} \;

# 3. Rotate logs immediately
logrotate -f /etc/logrotate.d/actionphase

# 4. Restart services if needed
docker-compose restart
```

### Need to Recover Old Logs
```bash
# Check Docker container logs (if container still exists)
docker logs <container-id> > recovered-logs.txt

# Check system journal
journalctl -u docker -since "2025-01-01" > docker-journal.log
```

---

## Quick Commands Cheat Sheet

```bash
# View live backend logs
docker-compose $COMPOSE_FILES logs -f backend

# Search for errors
docker-compose $COMPOSE_FILES logs backend | grep -i error

# View last 100 backend logs as JSON
tail -100 logs/backend/app.log | jq .

# Find all 500 errors in nginx
grep " 500 " logs/nginx/access.log

# Check log disk usage
du -sh logs/*

# Rotate logs now
logrotate -f /etc/logrotate.d/actionphase

# Clean old Docker resources
docker system prune -a

# Export logs for debugging
tar -czf logs-$(date +%Y%m%d).tar.gz logs/
```
