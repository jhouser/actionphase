# Build stage
FROM node:24-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Accept build arguments for Vite environment variables
# These are baked into the frontend bundle at build time
ARG VITE_HCAPTCHA_SITE_KEY
ARG VITE_HCAPTCHA_ENABLED=true

# Set environment variables for build (Vite reads VITE_* env vars)
ENV VITE_HCAPTCHA_SITE_KEY=$VITE_HCAPTCHA_SITE_KEY
ENV VITE_HCAPTCHA_ENABLED=$VITE_HCAPTCHA_ENABLED

# Faro RUM — baked into the bundle
ARG VITE_FARO_ENABLED=false
ARG VITE_FARO_URL=
ARG VITE_FARO_APP_NAME=actionphase
ARG VITE_ENVIRONMENT=production

ENV VITE_FARO_ENABLED=$VITE_FARO_ENABLED
ENV VITE_FARO_URL=$VITE_FARO_URL
ENV VITE_FARO_APP_NAME=$VITE_FARO_APP_NAME
ENV VITE_ENVIRONMENT=$VITE_ENVIRONMENT

# Comment threading depth limits — baked into the bundle.
# Must stay <= the backend's COMMENT_MAX_DEPTH, which caps the depth the API
# returns; a higher value here renders "Continue thread" for replies never fetched.
ARG VITE_COMMENT_MAX_DEPTH=5
ARG VITE_COMMENT_MAX_DEPTH_MOBILE=3

ENV VITE_COMMENT_MAX_DEPTH=$VITE_COMMENT_MAX_DEPTH
ENV VITE_COMMENT_MAX_DEPTH_MOBILE=$VITE_COMMENT_MAX_DEPTH_MOBILE

# Faro source map upload — consumed only by the Vite build plugin, not shipped to browser
ARG GRAFANA_FARO_API_KEY=
ARG GRAFANA_FARO_SOURCEMAP_ENDPOINT=
ARG GRAFANA_FARO_APP_ID=
ARG GRAFANA_FARO_STACK_ID=

ENV GRAFANA_FARO_API_KEY=$GRAFANA_FARO_API_KEY
ENV GRAFANA_FARO_SOURCEMAP_ENDPOINT=$GRAFANA_FARO_SOURCEMAP_ENDPOINT
ENV GRAFANA_FARO_APP_ID=$GRAFANA_FARO_APP_ID
ENV GRAFANA_FARO_STACK_ID=$GRAFANA_FARO_STACK_ID

# Build the application
RUN npm run build
# Generate legacy-only index.html for iOS < 16.
# Strips all type="module" scripts so old Safari never tries to load
# the modern bundle — it only sees the nomodule legacy bundle scripts.
RUN node -e " \
  const fs = require('fs'); \
  let html = fs.readFileSync('dist/index.html', 'utf8'); \
  html = html.replace(/<script type=\"module\"[^>]*>[^<]*<\/script>/g, ''); \
  html = html.replace(/<script type=\"module\"[^>]*\/>/g, ''); \
  html = html.replace(/(<script) nomodule (crossorigin id=\"vite-legacy-polyfill\")/g, '\$1 \$2'); \
  html = html.replace(/(<script) nomodule (crossorigin id=\"vite-legacy-entry\")/g, '\$1 \$2'); \
  html = html.replace(/<script nomodule>[^<]*<\/script>/g, ''); \
  fs.writeFileSync('dist/index.legacy.html', html); \
  console.log('Generated index.legacy.html'); \
  console.log(fs.readFileSync('dist/index.legacy.html', 'utf8')); \
"

# Runtime stage - serve with nginx
FROM nginx:alpine
# Copy built assets from builder
COPY --from=builder /app/dist /usr/share/nginx/html
# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf
# Expose port
EXPOSE 80
# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost/ || exit 1
# Start nginx
CMD ["nginx", "-g", "daemon off;"]