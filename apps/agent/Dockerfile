FROM node:24-bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends fonts-noto-cjk && rm -rf /var/lib/apt/lists/*
COPY package.json package-lock.json ./
COPY apps/agent/package.json apps/agent/package.json
COPY packages/shared/package.json packages/shared/package.json
RUN npm ci --workspace @anime/agent --omit=dev --ignore-scripts --no-audit
COPY apps/agent/src apps/agent/src
COPY packages/shared packages/shared
RUN mkdir -p /state /media && chown node:node /state /media
CMD ["sh","-ec","chown node:node /media && exec setpriv --reuid=node --regid=node --init-groups node apps/agent/src/server.ts"]
