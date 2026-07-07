FROM node:24-alpine AS build

WORKDIR /app

ARG VITE_USER_API_BASE_URL=http://localhost:31081
ARG VITE_MEDIA_API_BASE_URL=http://localhost:31080
ENV VITE_USER_API_BASE_URL=${VITE_USER_API_BASE_URL}
ENV VITE_MEDIA_API_BASE_URL=${VITE_MEDIA_API_BASE_URL}

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

