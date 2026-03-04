#!/usr/bin/env python3
"""
Script per importare dati ThermoProSensor in TimescaleDB.
Importa solo i dati non già presenti nel database.
"""

import csv
import sys
from datetime import datetime, timezone
from pathlib import Path
import psycopg2
from psycopg2.extras import execute_batch
import argparse


def parse_datetime(date_str: str, time_str: str) -> datetime:
    """
    Converte data e ora dal CSV in timestamp con timezone UTC.
    
    Args:
        date_str: Data in formato DD-MM-YYYY (es. "11-02-2026")
        time_str: Ora in formato H:MM (es. "0:00" o "15:30")
    
    Returns:
        datetime object con timezone UTC
    """
    # Rimuovi BOM se presente
    date_str = date_str.lstrip('\ufeff')
    
    # Parse date (DD-MM-YYYY)
    day, month, year = date_str.split('-')
    
    # Parse time (H:MM)
    hour, minute = time_str.split(':')
    
    # Crea datetime con timezone UTC
    return datetime(int(year), int(month), int(day), int(hour), int(minute), tzinfo=timezone.utc)


def read_csv_data(csv_path: Path):
    """
    Legge i dati dal CSV.
    
    Args:
        csv_path: Path del file CSV
        
    Yields:
        Tuple (timestamp, temperature, humidity)
    """
    with open(csv_path, 'r', encoding='utf-8-sig') as f:
        # Salta la prima riga (descrizione)
        next(f)
        
        reader = csv.DictReader(f)
        for row in reader:
            try:
                # Normalizza le chiavi rimuovendo BOM se presente
                row_clean = {k.lstrip('\ufeff'): v for k, v in row.items()}
                
                timestamp = parse_datetime(row_clean['Data'], row_clean['Ora'])
                temperature = float(row_clean['Temperatura_Celsius'])
                humidity = float(row_clean['Umidità_relativa(%)'])
                
                yield (timestamp, temperature, humidity)
            except Exception as e:
                print(f"Errore parsing riga {row}: {e}", file=sys.stderr)
                continue


def get_existing_timestamps(cursor, sensor_id: str = 'TP357S'):
    """
    Recupera tutti i timestamp già presenti nel database per il sensore specificato.
    
    Args:
        cursor: Database cursor
        sensor_id: ID del sensore
        
    Returns:
        Set di timestamp già presenti
    """
    cursor.execute(
        "SELECT time FROM thermo_sensor WHERE sensor_id = %s",
        (sensor_id,)
    )
    return {row[0] for row in cursor.fetchall()}


def import_data(csv_path: Path, db_config: dict, sensor_id: str = 'TP357S', batch_size: int = 1000):
    """
    Importa i dati dal CSV al database, evitando duplicati.
    
    Args:
        csv_path: Path del file CSV
        db_config: Dizionario con configurazione DB (host, database, user, password, port)
        sensor_id: ID del sensore
        batch_size: Numero di record da inserire per batch
    """
    # Connessione al database
    conn = psycopg2.connect(**db_config)
    cursor = conn.cursor()
    
    try:
        print("Recupero timestamp esistenti...")
        existing_timestamps = get_existing_timestamps(cursor, sensor_id)
        print(f"Trovati {len(existing_timestamps)} record esistenti nel database")
        
        print(f"Lettura dati da {csv_path}...")
        new_records = []
        skipped = 0
        
        for timestamp, temperature, humidity in read_csv_data(csv_path):
            if timestamp in existing_timestamps:
                skipped += 1
                continue
            
            new_records.append((timestamp, temperature, humidity, sensor_id))
        
        print(f"Record da importare: {len(new_records)}")
        print(f"Record già esistenti (saltati): {skipped}")
        
        if new_records:
            print("Importazione in corso...")
            execute_batch(
                cursor,
                """
                INSERT INTO thermo_sensor (time, temperature, humidity, sensor_id)
                VALUES (%s, %s, %s, %s)
                """,
                new_records,
                page_size=batch_size
            )
            conn.commit()
            print(f"✓ Importati {len(new_records)} record con successo!")
        else:
            print("Nessun nuovo record da importare")
        
    except Exception as e:
        conn.rollback()
        print(f"Errore durante l'importazione: {e}", file=sys.stderr)
        raise
    finally:
        cursor.close()
        conn.close()


def main():
    parser = argparse.ArgumentParser(
        description='Importa dati ThermoProSensor in TimescaleDB'
    )
    parser.add_argument(
        'csv_file',
        type=Path,
        help='Path del file CSV da importare'
    )
    parser.add_argument(
        '--host',
        default='localhost',
        help='Database host (default: localhost)'
    )
    parser.add_argument(
        '--port',
        type=int,
        default=5435,
        help='Database port (default: 5435)'
    )
    parser.add_argument(
        '--database',
        default='termostato',
        help='Database name (default: termostato)'
    )
    parser.add_argument(
        '--user',
        default='termostato',
        help='Database user (default: termostato)'
    )
    parser.add_argument(
        '--password',
        default='IkyipShoownAd2',
        help='Database password'
    )
    parser.add_argument(
        '--sensor-id',
        default='TP357S',
        help='Sensor ID (default: TP357S)'
    )
    parser.add_argument(
        '--batch-size',
        type=int,
        default=1000,
        help='Batch size per inserimenti (default: 1000)'
    )
    
    args = parser.parse_args()
    
    if not args.csv_file.exists():
        print(f"Errore: File {args.csv_file} non trovato", file=sys.stderr)
        sys.exit(1)
    
    db_config = {
        'host': args.host,
        'port': args.port,
        'database': args.database,
        'user': args.user,
        'password': args.password
    }
    
    print(f"Importazione da: {args.csv_file}")
    print(f"Database: {args.user}@{args.host}:{args.port}/{args.database}")
    print(f"Sensor ID: {args.sensor_id}")
    print("-" * 60)
    
    import_data(args.csv_file, db_config, args.sensor_id, args.batch_size)


if __name__ == '__main__':
    main()
