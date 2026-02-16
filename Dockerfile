# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/scrappy ./

FROM debian:bookworm-slim

# Install base packages and fonts available in Debian repos
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    chromium \
    ca-certificates \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-lato \
    fonts-open-sans \
    fontconfig \
    curl \
  && rm -rf /var/lib/apt/lists/*

# Download Google Fonts from the official Google Fonts repo
ENV FONT_BASE="https://raw.githubusercontent.com/google/fonts/main/ofl"
RUN mkdir -p /usr/share/fonts/google && cd /usr/share/fonts/google \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/oswald/Oswald%5Bwght%5D.ttf" -o Oswald.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/sourcesans3/SourceSans3%5Bwght%5D.ttf" -o SourceSans3.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/sourcesans3/SourceSans3-Italic%5Bwght%5D.ttf" -o SourceSans3-Italic.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/nunito/Nunito%5Bwght%5D.ttf" -o Nunito.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/nunito/Nunito-Italic%5Bwght%5D.ttf" -o Nunito-Italic.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/karla/Karla%5Bwght%5D.ttf" -o Karla.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/karla/Karla-Italic%5Bwght%5D.ttf" -o Karla-Italic.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/playfairdisplay/PlayfairDisplay%5Bwght%5D.ttf" -o PlayfairDisplay.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/playfairdisplay/PlayfairDisplay-Italic%5Bwght%5D.ttf" -o PlayfairDisplay-Italic.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/montserrat/Montserrat%5Bwght%5D.ttf" -o Montserrat.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/montserrat/Montserrat-Italic%5Bwght%5D.ttf" -o Montserrat-Italic.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/merriweather/Merriweather%5Bopsz,wdth,wght%5D.ttf" -o Merriweather.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/merriweather/Merriweather-Italic%5Bopsz,wdth,wght%5D.ttf" -o Merriweather-Italic.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/cinzel/Cinzel%5Bwght%5D.ttf" -o Cinzel.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/raleway/Raleway%5Bwght%5D.ttf" -o Raleway.ttf \
  && curl -fsSL --retry 5 --retry-delay 1 "${FONT_BASE}/raleway/Raleway-Italic%5Bwght%5D.ttf" -o Raleway-Italic.ttf \
  && fc-cache -f -v \
  && apt-get purge -y curl \
  && apt-get autoremove -y

RUN groupadd --system scrappy \
  && useradd --system --gid scrappy --create-home --home-dir /home/scrappy scrappy \
  && mkdir -p /app \
  && chown -R scrappy:scrappy /app /home/scrappy

ENV SCRAPPY_CHROME_BIN=/usr/bin/chromium
ENV SCRAPPY_ADDR=:3000

WORKDIR /app
COPY --from=builder /out/scrappy /usr/local/bin/scrappy

USER scrappy

EXPOSE 3000
CMD ["scrappy"]
