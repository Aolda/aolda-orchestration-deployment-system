FROM node:22-alpine AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

ARG VITE_API_BASE_URL=/api
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ARG VITE_AODS_PLATFORM_ADMIN_AUTHORITIES=aods:platform:admin
ENV VITE_AODS_PLATFORM_ADMIN_AUTHORITIES=${VITE_AODS_PLATFORM_ADMIN_AUTHORITIES}

COPY . .
RUN npm run build

FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
