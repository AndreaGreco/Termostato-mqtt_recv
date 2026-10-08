-- Script per inizializzare Tern su un database esistente già aggiornato
-- Esegui questo script MANUALMENTE sul database esistente prima di avviare i container
--
-- La versione 6 è lo schema dei database creati prima di Tern: NON alzarla
-- quando si aggiungono migrazioni, altrimenti Tern salterebbe le migrazioni
-- successive (007, 008, 009, ...) su quei database.

-- Crea la tabella schema_version usata da Tern per tracciare le migrazioni
-- Tern usa UNA SOLA riga con il numero di versione corrente
CREATE TABLE IF NOT EXISTS schema_version (
    version integer
);

-- Inserisce la versione corrente se la tabella è vuota, altrimenti aggiorna
INSERT INTO schema_version (version)
SELECT 6
WHERE NOT EXISTS (SELECT 1 FROM schema_version);

UPDATE schema_version SET version = 6;

-- Verifica stato
SELECT * FROM schema_version;
