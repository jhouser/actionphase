# SSL Bootstrap Guide - Quick Start

This guide solves the chicken-and-egg problem: nginx won't start without SSL certificates, but you need nginx running to get SSL certificates.

## The Solution

We use a two-phase approach:
1. **Phase 1:** Start nginx with HTTP-only config (no SSL required)
2. **Phase 2:** Get SSL certificates, then switch to full SSL config

## Quick Start (On Production Server)

### Prerequisites

1. **DNS is configured** - Your domain's A record points to this server
   ```bash
   dig +short yourdomain.com
   # Should return your server's IP
   ```

2. **Firewall allows ports 80 and 443**
   ```bash
   sudo ufw allow 80
   sudo ufw allow 443
   ```

3. **Docker and docker-compose are installed**

### Step 1: Clone and Configure

```bash
# Clone repository
cd /opt
git clone https://github.com/yourusername/actionphase.git
cd actionphase

# Create .env file
cp .env.example .env
nano .env
```

Update `.env`:
```env
DOMAIN=yourdomain.com
JWT_SECRET=your-super-secret-jwt-key-at-least-32-chars
ENVIRONMENT=production
```

### Step 2: Bootstrap Nginx (HTTP-only)

This starts nginx WITHOUT SSL certificates:

```bash
./scripts/bootstrap-nginx.sh
```

**What this does:**
- Stops any existing nginx
- Creates required Docker networks and volumes
- Starts nginx with HTTP-only configuration
- Nginx listens on port 80 for Let's Encrypt validation

**Expected output:**
```
========================================
   Nginx Bootstrap (HTTP-only)
========================================

Stopping nginx if running...
Starting nginx with HTTP-only configuration...
✓ Nginx started successfully on port 80
✓ Nginx is responding

========================================
✓ Bootstrap Complete!
========================================
```

### Step 3: Get SSL Certificates

```bash
./scripts/setup-ssl.sh yourdomain.com admin@yourdomain.com
```

**What this does:**
1. Verifies nginx is running on port 80
2. Generates DH parameters (2-5 minutes)
3. Obtains SSL certificate from Let's Encrypt
4. **Switches nginx to full SSL configuration**
5. Restarts nginx with SSL support
6. Sets up automatic renewal via cron

**Expected output:**
```
========================================
   ActionPhase SSL Setup
========================================

Domain: yourdomain.com
Email: admin@yourdomain.com

Generating DH parameters...
✓ DH parameters generated
Preparing nginx for certificate validation...
✓ Nginx is already running
Obtaining SSL certificate from Let's Encrypt...
✓ SSL certificate obtained successfully!
Switching to full SSL nginx configuration...
✓ Nginx started with SSL configuration
✓ HTTPS is working!
✓ SSL Setup Complete!
```

### Step 4: Start Full Application Stack

```bash
# Start all services (db, backend, frontend, etc.)
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## Verification

Test your site:

```bash
# Test HTTP → HTTPS redirect
curl -I http://yourdomain.com
# Should show 301 redirect to https://

# Test HTTPS
curl -I https://yourdomain.com
# Should show 200 OK

# Check SSL certificate
./scripts/check-ssl.sh yourdomain.com
```

Or visit in browser:
- https://yourdomain.com (should work with valid certificate)
- http://yourdomain.com (should redirect to HTTPS)

## Troubleshooting

### Problem: bootstrap-nginx.sh fails with "network not found"

**Solution:** Start the database first to create the network:
```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d db
sleep 5
./scripts/bootstrap-nginx.sh
```

### Problem: Certificate validation fails

**Check DNS propagation:**
```bash
dig +short yourdomain.com
# Must return your server's IP
```

**Check port 80 is accessible:**
```bash
# From another machine:
curl http://yourdomain.com/.well-known/acme-challenge/
# Should return 404 (nginx is serving, but no challenge file)
```

**Check nginx logs:**
```bash
docker logs actionphase-nginx
```

### Problem: Nginx won't start after SSL setup

**Check certificate was created:**
```bash
docker exec actionphase-certbot ls -la /etc/letsencrypt/live/yourdomain.com/
```

**Check dhparam file exists:**
```bash
ls -la ssl/dhparam.pem
```

**View detailed error:**
```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml logs nginx
```

### Problem: Need to start over

**Complete reset:**
```bash
# Stop everything
docker-compose -f docker-compose.yml -f docker-compose.prod.yml down

# Remove nginx
docker rm -f actionphase-nginx

# Remove volumes (WARNING: This deletes SSL certificates!)
docker volume rm actionphase_letsencrypt actionphase_certbot-webroot

# Start fresh
./scripts/bootstrap-nginx.sh
./scripts/setup-ssl.sh yourdomain.com admin@yourdomain.com
```

## How It Works

### Phase 1: Bootstrap (HTTP-only)

The `bootstrap-nginx.sh` script starts nginx with this simple config:
- Listens on port 80 only
- Serves `.well-known/acme-challenge/` for Let's Encrypt
- Proxies requests to backend/frontend
- **No SSL certificates required**

### Phase 2: SSL Setup

The `setup-ssl.sh` script:
1. Detects nginx is running (or starts it)
2. Runs certbot to get SSL certificates
3. **Stops the HTTP-only nginx**
4. Switches to full SSL nginx configuration (via docker-compose)
5. nginx now:
   - Redirects HTTP → HTTPS
   - Serves HTTPS with valid certificates
   - Has full security headers

## Automatic Renewal

After setup, certificates auto-renew via cron job.

> `scripts/renew-ssl.sh` is **generated by `setup-ssl.sh`**, not committed to the
> repo — you will not find it in a fresh checkout. It appears on the server once
> setup has run.

**Check cron is configured:**
```bash
crontab -l | grep renew-ssl
# Should show: 0 0,12 * * * /opt/actionphase/scripts/renew-ssl.sh
```

**Test renewal (dry run):**
```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml run --rm --entrypoint "" certbot \
    certbot renew --dry-run \
    --webroot \
    --webroot-path /var/www/certbot
```

## Manual Renewal

If needed, manually renew:

```bash
./scripts/setup-ssl.sh yourdomain.com admin@yourdomain.com
# Select "Yes" when asked if you want to renew
```

## Architecture Summary

```
Internet
   ↓
Port 80/443
   ↓
Nginx (actionphase-nginx)
   ├─→ /api/* → Backend (internal port 3000)
   ├─→ /* → Frontend (internal port 80)
   └─→ /.well-known/acme-challenge/ → Certbot validation
```

**Docker Volumes:**
- `actionphase_letsencrypt` - SSL certificates
- `actionphase_certbot-webroot` - Let's Encrypt challenge files

**Network:**
- `actionphase_actionphase-network` - Internal Docker network for all services

## Related Documentation

- [Route53 DNS Setup Guide](./ROUTE53_SSL_SETUP.md)
- [Production Env Checklist](./PRODUCTION_ENV_CHECKLIST.md)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
