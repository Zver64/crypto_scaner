FROM node:24-alpine AS build

WORKDIR /src

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY . .
ARG APP_VERSION=dev
RUN VITE_APP_VERSION="$APP_VERSION" npm run build

FROM nginxinc/nginx-unprivileged:1.29-alpine AS production

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 8080
