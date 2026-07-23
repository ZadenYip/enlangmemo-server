FROM node:24-alpine AS build
WORKDIR /app

# Enable corepack to use pnpm
RUN corepack enable

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm build

FROM nginx:1.30.4-alpine

COPY web/docker/nginx.conf /etc/nginx/conf.d/default.conf

# Angular 新版 application builder 常见输出路径是 dist/<project>/browser
COPY --from=build /app/dist/enlangmemo-web/browser /usr/share/nginx/html

EXPOSE 80