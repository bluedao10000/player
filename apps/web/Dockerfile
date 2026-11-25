FROM node:24-bookworm-slim AS source
WORKDIR /app
COPY package.json package-lock.json tsconfig.base.json ./
COPY apps/web/package.json apps/web/package.json
COPY packages/shared/package.json packages/shared/package.json
RUN npm ci --workspace @anime/web --include-workspace-root --no-audit
COPY apps/web apps/web
COPY apps/ios/server apps/ios/server
COPY packages/shared packages/shared

FROM source AS reader-build
RUN npm run build:reader-api -w @anime/web

FROM node:24-bookworm-slim AS reader-api
WORKDIR /app
COPY --from=reader-build /app/node_modules/srvx ./node_modules/srvx
COPY --from=reader-build /app/apps/web/dist/reader-api/reader-api.js ./reader-api.mjs
ENV READER_API_HOST=0.0.0.0
USER node
CMD ["node", "reader-api.mjs"]

FROM source AS site-build
RUN npm run build:web

FROM nginx:stable-alpine AS site
RUN adduser -D -u 1000 folio-edge
COPY --from=site-build /app/apps/web/dist/site/ /usr/share/nginx/html/
