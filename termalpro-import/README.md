# ThermoProSensor Import

Script per importare dati da ThermoProSensor in TimescaleDB.

## 🐳 Uso con Docker (Consigliato)

### Import Automatico

1. Copia il file CSV da importare nella root del progetto con nome `import-temperature.csv`
2. Avvia i servizi:

```bash
cd /home/agreco/Develop/Altri/esp32/Termostato/mqtt_recv
docker-compose up
```

Il servizio `thermopro-import`:
- ✓ Attende che il database sia pronto
- ✓ Controlla se esiste `import-temperature.csv`
- ✓ Importa i dati (saltando duplicati)
- ✓ **Elimina automaticamente il file dopo l'import**
- ✓ Termina dopo l'operazione

### Personalizzazione

È possibile modificare il sensor ID tramite variabile d'ambiente nel [docker-compose.yml](../docker-compose.yml):

```yaml
environment:
  - SENSOR_ID=TP357S  # Cambia questo valore se necessario
```

## 🔧 Uso Manuale con Python

### Prerequisiti

1. Database TimescaleDB attivo (via docker-compose)
2. Python 3.x installato
3. Tabella `thermo_sensor` creata (migration 005)

## Installazione

```bash
cd termalpro-import
pip install -r requirements.txt
```

## Applicare la Migration

Prima di importare i dati, applicare la migration al database:

```bash
cd ../go-client
# Se usi tern per le migrations
tern migrate -m migrations
```

## Utilizzo

### Uso Base

```bash
python import_thermopro.py /path/to/ThermoProSensor_export.csv
```

### Con Parametri Personalizzati

```bash
python import_thermopro.py /path/to/file.csv \
  --host localhost \
  --port 5435 \
  --database termostato \
  --user termostato \
  --password IkyipShoownAd2 \
  --sensor-id TP357S \
  --batch-size 1000
```

### Parametri

- `csv_file`: (richiesto) Path del file CSV da importare
- `--host`: Host del database (default: localhost)
- `--port`: Porta del database (default: 5435)
- `--database`: Nome database (default: termostato)
- `--user`: User database (default: termostato)
- `--password`: Password database
- `--sensor-id`: ID del sensore (default: TP357S)
- `--batch-size`: Dimensione batch per inserimenti (default: 1000)

## Esempio con il File Allegato

```bash
python import_thermopro.py "/home/agreco/Scaricati/ThermoProSensor_export_TP357S _15022026(1).csv"
```

## Caratteristiche

- ✓ Converte automaticamente data e ora in timestamp
- ✓ Evita duplicati (controlla timestamp esistenti)
- ✓ Import batch per performance ottimali
- ✓ Gestione errori su righe malformate
- ✓ Report dettagliato dell'import

## Formato CSV Supportato

```
Marcatura temporale per frequenza campione ogni 1min
Data,Ora,Temperatura_Celsius,Umidità_relativa(%)
11-02-2026,0:00,18.1,69
...
```

- Data: formato DD-MM-YYYY
- Ora: formato H:MM
- La prima riga (descrizione) viene saltata automaticamente
