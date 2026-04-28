###############  Stage 1 – Go build  ###############
FROM golang:1.26-alpine AS backend-build

ARG BUILD_VERSION=dev
ARG BUILD_TIME=unknown
ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /backend
COPY backend/ .

ENV CGO_ENABLED=0
RUN GOOS="${TARGETOS}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
  go build -ldflags "-X 'magpie/internal/app/version.buildVersion=${BUILD_VERSION}' -X 'magpie/internal/app/version.builtAt=${BUILD_TIME}'" -o server ./cmd/magpie

############ Stage 2 – grab Chromium’s shared libraries ############
FROM debian:bookworm-slim AS chromium-deps

RUN apt-get update && apt-get install -y --no-install-recommends \
      chromium \
      libglib2.0-0  libgtk-3-0  libnss3  libasound2 \
      libatk-bridge2.0-0  libatk1.0-0  libcups2  libdrm2  libgbm1 \
      libx11-xcb1  libxcomposite1  libxdamage1  libxrandr2  libxkbcommon0 \
      fonts-liberation  ca-certificates  xdg-utils \
  && rm -rf /var/lib/apt/lists/*

############ Stage 3 – tiny runtime image ##########################
FROM gcr.io/distroless/base-debian12:nonroot

# Copy architecture-specific Chromium runtime libraries.
COPY --from=chromium-deps /lib /lib
COPY --from=chromium-deps /usr/lib /usr/lib
COPY --from=chromium-deps /usr/share/fonts /usr/share/fonts
COPY --from=chromium-deps /usr/share/chromium /usr/share/chromium

WORKDIR /app
COPY --from=backend-build /backend/server .

USER 65532:65532

EXPOSE 5656
ENTRYPOINT ["./server"]
