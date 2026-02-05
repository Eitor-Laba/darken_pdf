# syntax=docker/dockerfile:1
FROM python:3.12-alpine AS build

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

RUN apk add --no-cache \
    build-base \
    mupdf-dev \
    pkgconf \
    python3-dev

COPY requirements.txt ./
RUN python -m venv /venv \
    && /venv/bin/pip install --no-cache-dir -r requirements.txt


FROM python:3.12-alpine

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apk add --no-cache \
    mupdf \
    ca-certificates

COPY --from=build /venv /venv
ENV PATH="/venv/bin:$PATH"

COPY . .

EXPOSE 8080

CMD ["python", "app.py"]