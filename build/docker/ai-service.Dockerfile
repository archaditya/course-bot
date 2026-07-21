FROM python:3.11-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements FIRST and install dependencies for Docker layer caching
COPY apps/ai-service/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Copy python app source AFTER dependencies layer is cached
COPY apps/ai-service/ ./

EXPOSE 8000
CMD ["uvicorn", "api.server:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "2"]
