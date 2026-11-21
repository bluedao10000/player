FROM node:24-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json tsconfig.base.json ./
COPY apps/web/package.json apps/web/package.json
COPY packages/shared/package.json packages/shared/package.json
RUN npm ci --workspace @anime/web --include-workspace-root --no-audit
COPY apps/web apps/web
COPY apps/ios/server apps/ios/server
COPY packages/shared packages/shared
RUN npm run build -w @anime/web
RUN npm prune --workspace @anime/web --omit=dev --no-audit

FROM node:24-bookworm-slim
WORKDIR /app
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/packages/shared ./packages/shared
COPY --from=build /app/apps/web ./apps/web
WORKDIR /app/apps/web
USER node
ENV WEB_HOST=0.0.0.0
CMD ["node", "serve.mjs"]
