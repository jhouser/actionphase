# Production Logging Strategy

> **Status: Option 2 was implemented (2025-11-08).** This document is kept for
> the decision rationale and the CloudWatch comparison. The "Recommended
> Solutions" below are **not open questions** — `docker-compose.logging.yml` and
> the logrotate config in `terraform/user-data.sh` are live.
>
> **Where the shipped config differs from the proposal below:**
>
> | Setting | Proposed here | Actually deployed |
> |---|---|---|
> | logrotate `rotate` | 30 days | **14 days** (`terraform/user-data.sh:181`) |
> | logrotate scope | `/opt/actionphase/logs/**/*.log` | Per-directory blocks: `backend/`, `nginx/`, `frontend/`, plus `backups/` weekly ×4 |
> | `db` container | not covered | `max-size: 20m`, `max-file: 5` |
> | backend / nginx | `100m` × 10 | matches |
> | frontend | `50m` × 5 | matches |
>
> Read `docker-compose.logging.yml` and `terraform/user-data.sh` for current
> values; the YAML in Option 2 is the original proposal, not a mirror of what
> runs.


## Current State (Container Logs)

### JSON File Logging (Current)
The production deployment uses Docker's `json-file` logging driver with rotation:

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "50m"
    max-file: "5"
```

**Location**: `/var/lib/docker/containers/<container-id>/<container-id>-json.log`

**Pros**:
- Built-in log rotation prevents disk fill-up
- No additional services required
- Low overhead

**Cons**:
- ❌ **Logs lost when container is removed** (not just restarted)
- ❌ No centralized log aggregation
- ❌ Can't search across containers
- ❌ No long-term retention
- ❌ Difficult to troubleshoot historical issues

### Structured Application Logging (Current)
Backend uses structured JSON logging with correlation IDs:

```go
logger.Info(ctx, "Request processed",
    "user_id", userID,
    "duration_ms", duration,
    "status", status)
```

**Location**: Container STDOUT → Docker JSON logs
**Format**: JSON with timestamps, correlation IDs, structured fields

---

## Recommended Solutions

### Option 1: AWS CloudWatch Logs (Recommended for AWS Deployments)

**Best for**: Production AWS deployments, teams familiar with AWS ecosystem

#### Advantages
✅ Native AWS integration
✅ Searchable with CloudWatch Logs Insights
✅ Can trigger CloudWatch Alarms
✅ Integration with AWS services (Lambda, SNS, etc.)
✅ Long-term retention (configurable)
✅ No infrastructure to manage

#### Costs
- **Ingestion**: $0.50 per GB
- **Storage**: $0.03 per GB/month
- **Insights queries**: $0.005 per GB scanned

**Estimated monthly cost** (assuming 10GB logs/month):
- Ingestion: $5
- Storage (30 days): $0.30
- Queries (10 GB scanned): $0.05
- **Total: ~$5.35/month**

#### Implementation

1. **Install CloudWatch agent in user-data.sh**:
```bash
# Install CloudWatch Logs agent
echo "Installing CloudWatch Logs agent..."
wget -q https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/amd64/latest/amazon-cloudwatch-agent.deb
dpkg -i -E ./amazon-cloudwatch-agent.deb
```

2. **Update docker-compose.prod.yml**:
```yaml
services:
  backend:
    logging:
      driver: "awslogs"
      options:
        awslogs-region: "${AWS_REGION:-us-east-1}"
        awslogs-group: "/actionphase/backend"
        awslogs-stream: "backend-{instance_id}"
        awslogs-create-group: "true"

  frontend:
    logging:
      driver: "awslogs"
      options:
        awslogs-region: "${AWS_REGION:-us-east-1}"
        awslogs-group: "/actionphase/frontend"
        awslogs-stream: "frontend-{instance_id}"
        awslogs-create-group: "true"

  nginx:
    logging:
      driver: "awslogs"
      options:
        awslogs-region: "${AWS_REGION:-us-east-1}"
        awslogs-group: "/actionphase/nginx"
        awslogs-stream: "nginx-{instance_id}"
        awslogs-create-group: "true"
```

3. **Update IAM role** (terraform/main.tf):
```hcl
resource "aws_iam_role_policy" "cloudwatch_logs" {
  name = "actionphase-cloudwatch-logs"
  role = aws_iam_role.ec2_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:*:*:log-group:/actionphase/*"
      }
    ]
  })
}
```

4. **Query logs with CloudWatch Insights**:
```
fields @timestamp, correlation_id, message, user_id
| filter @logStream like /backend/
| filter level = "error"
| sort @timestamp desc
| limit 100
```

---

### Option 2: Local Volume Persistence (Recommended for Self-Hosted)

**Best for**: Non-AWS deployments, cost-sensitive deployments, self-hosted

#### Implementation

1. **Create docker-compose.logging.yml override**:
```yaml
# docker-compose.logging.yml
# Use with: docker-compose -f docker-compose.yml -f docker-compose.prod.yml -f docker-compose.logging.yml up -d

services:
  backend:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"
        labels: "service,environment"
    volumes:
      # Persist logs to host
      - ./logs/backend:/var/log/backend
    # Override CMD to also log to file
    command: >
      sh -c "
        /app/backend 2>&1 | tee -a /var/log/backend/app.log
      "

  frontend:
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "5"
    volumes:
      - ./logs/frontend:/var/log/nginx

  nginx:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"
    volumes:
      # Persist nginx access and error logs
      - ./logs/nginx:/var/log/nginx
```

2. **Update logrotate configuration**:
```bash
# /etc/logrotate.d/actionphase-docker
/opt/actionphase/logs/**/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 ubuntu ubuntu
    sharedscripts
    postrotate
        # Signal containers to reopen log files if needed
        docker kill --signal=USR1 actionphase-nginx 2>/dev/null || true
    endscript
}
```

3. **Add to user-data.sh**:
```bash
# Create logs directory
mkdir -p /opt/actionphase/logs/{backend,frontend,nginx}
chown -R ubuntu:ubuntu /opt/actionphase/logs

# Set up log rotation for Docker logs
cat > /etc/logrotate.d/actionphase-docker << 'EOF'
/opt/actionphase/logs/**/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 ubuntu ubuntu
}
EOF
```

---

### Option 3: Hybrid Approach (Recommended)

Combine both approaches:
- **Critical production logs** → CloudWatch (backend errors, security events)
- **Debug/verbose logs** → Local volumes (full request logs, nginx access logs)

```yaml
services:
  backend:
    # Send to CloudWatch for searchability
    logging:
      driver: "awslogs"
      options:
        awslogs-group: "/actionphase/backend"
        awslogs-create-group: "true"
    # Also write to local file for debugging
    volumes:
      - ./logs/backend:/var/log/backend
    command: >
      sh -c "/app/backend 2>&1 | tee -a /var/log/backend/app.log"
```

---

## Accessing Container Logs

### Current System
```bash
# View live logs
docker-compose logs -f backend

# View specific container
docker logs actionphase-backend -f

# View last 100 lines
docker logs actionphase-backend --tail 100

# View logs since timestamp
docker logs actionphase-backend --since 2025-01-01T00:00:00
```

### With CloudWatch
```bash
# Install AWS CLI on your local machine
aws logs tail /actionphase/backend --follow

# Search logs
aws logs filter-log-events \
  --log-group-name /actionphase/backend \
  --filter-pattern "ERROR" \
  --start-time $(date -d '1 hour ago' +%s)000

# Use CloudWatch Insights (via AWS Console)
```

### With Volume Persistence
```bash
# SSH to server
ssh ubuntu@your-server.com

# View logs
tail -f /opt/actionphase/logs/backend/app.log

# Search logs
grep "correlation_id=abc123" /opt/actionphase/logs/backend/app.log

# Analyze with jq (for JSON logs)
tail -100 /opt/actionphase/logs/backend/app.log | jq 'select(.level == "error")'
```

---

## Monitoring and Alerts

### CloudWatch Alarms (with CloudWatch Logs)

Create alarms for critical events:

```hcl
# terraform/monitoring.tf
resource "aws_cloudwatch_log_metric_filter" "backend_errors" {
  name           = "backend-errors"
  log_group_name = "/actionphase/backend"
  pattern        = "[time, request_id, level=ERROR, ...]"

  metric_transformation {
    name      = "BackendErrors"
    namespace = "ActionPhase"
    value     = "1"
  }
}

resource "aws_cloudwatch_metric_alarm" "backend_error_rate" {
  alarm_name          = "actionphase-backend-error-rate"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "BackendErrors"
  namespace           = "ActionPhase"
  period              = 300  # 5 minutes
  statistic           = "Sum"
  threshold           = 10
  alarm_description   = "Backend error rate is too high"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
```

### Log Monitoring Script (for Volume Persistence)

Create `/opt/actionphase/scripts/check-logs.sh`:

```bash
#!/bin/bash

# Check for error rate spikes
ERROR_COUNT=$(grep -c '"level":"error"' /opt/actionphase/logs/backend/app.log | tail -1000)

if [ "$ERROR_COUNT" -gt 50 ]; then
    echo "HIGH ERROR RATE: $ERROR_COUNT errors in last 1000 lines"
    # Send alert via Slack webhook
    curl -X POST "$SLACK_WEBHOOK_URL" \
        -H 'Content-Type: application/json' \
        -d "{\"text\":\":rotating_light: Backend error rate spike: $ERROR_COUNT errors\"}"
fi

# Check for disk space
LOGS_SIZE=$(du -sm /opt/actionphase/logs | cut -f1)
if [ "$LOGS_SIZE" -gt 5000 ]; then  # 5GB
    echo "WARNING: Logs directory is ${LOGS_SIZE}MB"
fi
```

Add to cron:
```bash
# Check logs every 15 minutes
*/15 * * * * ubuntu /opt/actionphase/scripts/check-logs.sh >> /var/log/log-monitor.log 2>&1
```

---

## Log Retention Policy

### CloudWatch Logs
```bash
# Set retention via AWS CLI
aws logs put-retention-policy \
  --log-group-name /actionphase/backend \
  --retention-in-days 30

# Or via Terraform
resource "aws_cloudwatch_log_group" "backend" {
  name              = "/actionphase/backend"
  retention_in_days = 30
}
```

### Local Volumes
Handled by logrotate configuration (**14 days**; see `terraform/user-data.sh`).

---

## Cost-Benefit Analysis

| Solution | Monthly Cost | Pros | Cons |
|----------|-------------|------|------|
| **JSON File (Current)** | $0 | Simple, no dependencies | Logs lost on container removal |
| **Local Volumes** | $0 | Persistent, searchable locally | Limited retention, manual management |
| **CloudWatch Logs** | ~$5-10 | Fully managed, searchable, alerts | Requires AWS, costs money |
| **Hybrid** | ~$3-5 | Best of both worlds | More complex setup |

---

## Recommendation

**For ActionPhase production deployment:**

1. **Start with Option 2 (Local Volumes)** - Zero cost, covers basic needs
2. **Upgrade to Option 3 (Hybrid)** when:
   - Need centralized logging across multiple instances
   - Want CloudWatch Alarms integration
   - Debugging production issues frequently
   - Budget allows $5-10/month for logging

**Implementation checklist:**
- [x] Add cron jobs for disk/SSL monitoring (done in user-data.sh)
- [ ] Choose logging strategy (Option 2 or 3)
- [ ] Update docker-compose.prod.yml with logging configuration
- [ ] Update user-data.sh to create log directories
- [ ] Configure logrotate for log retention
- [ ] Set up monitoring/alerting for log volume
- [ ] Document runbook for log access procedures

---

## Structured Logging Best Practices

The backend already uses structured logging, but ensure:

1. **Always include correlation IDs**:
```go
logger.Info(ctx, "Processing request", "user_id", userID)
```

2. **Use appropriate log levels**:
   - ERROR: Failures requiring attention
   - WARN: Unusual conditions
   - INFO: Important business events
   - DEBUG: Detailed debugging (disabled in production)

3. **Log security events**:
```go
logger.Warn(ctx, "Failed login attempt",
    "username", username,
    "ip_address", ipAddr)
```

4. **Don't log sensitive data**:
   - ❌ Passwords, tokens, API keys
   - ❌ Full credit card numbers
   - ❌ Social security numbers
   - ✅ User IDs, correlation IDs, sanitized inputs

---

## Next Steps

1. Review logging strategy options with team
2. Choose implementation (recommend Option 2 to start)
3. Test log persistence in staging environment
4. Update production deployment
5. Verify log retention and rotation working
6. Set up monitoring/alerts for log volume
