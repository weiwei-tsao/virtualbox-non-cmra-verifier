# =============================================================
# Stage 1: Build — pnpm workspace build of React + Vite SPA
# =============================================================
FROM node:20-alpine AS builder

WORKDIR /app

# Enable pnpm via corepack (project uses pnpm workspace with root pnpm-lock.yaml).
RUN corepack enable

# Copy workspace manifests and lockfile before source to maximise cache reuse.
# All three files live at the repo root, not inside apps/web/.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/

# Install only this workspace member's dependencies.
# --frozen-lockfile: exact versions, same as CI.
# --filter: matches "name" in apps/web/package.json → "us-virtual-address-verifier"
RUN pnpm install --frozen-lockfile --filter=us-virtual-address-verifier

# Copy source after install so dependency layer stays cached on source-only changes.
COPY apps/web/ ./apps/web/

# VITE_API_BASE_URL is a build-time variable baked into the JS bundle by Vite.
# Variable name comes from apps/web/services/api.ts:3 and hooks/useCSVExport.ts:4.
# Value must be browser-accessible (not the container-internal http://api:8080).
ARG VITE_API_BASE_URL
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

RUN pnpm --filter us-virtual-address-verifier build

# =============================================================
# Stage 2: Runtime — Nginx serves the static SPA
# =============================================================
FROM nginx:1.27-alpine

RUN rm -rf /usr/share/nginx/html/*

# Vite outputs the build to apps/web/dist.
COPY --from=builder /app/apps/web/dist /usr/share/nginx/html

# SPA routing: unmatched paths fall back to index.html (prevents refresh 404).
# Static assets get long-lived cache headers.
RUN printf 'server {\n\
    listen 80;\n\
    server_name localhost;\n\
    root /usr/share/nginx/html;\n\
    index index.html;\n\
    location / {\n\
        try_files $uri $uri/ /index.html;\n\
    }\n\
    location ~* \\.(js|css|png|ico|svg|woff2?)$ {\n\
        expires 1y;\n\
        add_header Cache-Control "public, immutable";\n\
    }\n\
}\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
