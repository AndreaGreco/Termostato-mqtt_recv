FROM python:3.11-slim

WORKDIR /app

# Installa postgresql-client per il health check
RUN apt-get update && apt-get install -y postgresql-client && rm -rf /var/lib/apt/lists/*

# Installa le dipendenze
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copia lo script e l'entrypoint
COPY import_thermopro.py .
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Crea directory per il file CSV
RUN mkdir -p /data

ENTRYPOINT ["/app/entrypoint.sh"]
