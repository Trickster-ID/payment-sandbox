FROM golang:1.26-alpine3.22 AS builder

ARG REPOSITORY_NAME

WORKDIR /${REPOSITORY_NAME}/
COPY . /${REPOSITORY_NAME}/

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /${REPOSITORY_NAME}/${REPOSITORY_NAME} ./app/cmd/

FROM alpine:3.22

# Read build arguments
ARG REPOSITORY_NAME

WORKDIR /srv/app

# Copy zone info
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
# Copy bin file from builder
COPY --from=builder /${REPOSITORY_NAME}/${REPOSITORY_NAME} app-bin
# Copy .env from builder
COPY --from=builder /${REPOSITORY_NAME}/.env.${APP_ENV} .env

EXPOSE 8000 8001

# Use ENTRYPOINT and CMD to set the command and its arguments
# WARNING! : change service name to related git repository
ENTRYPOINT ["/srv/app/app-bin"]