# Specifica: core dump dei termostati via MQTT

Stato: **da implementare** nel `go-client`. Viene dopo `SPEC-multi-device.md`
(già implementata, `e111a17`): usa lo stesso `topicPrefix` e lo stesso `<dev>`.
Il firmware è pronto (Termostato Firmware, `main/coredump_mqtt.c`).

## Il problema

Quando il termostato va in panic salva un core dump (ELF, tipicamente 20–100 KB)
nella partizione `coredump` della flash. Oggi lo si legge solo con
`GET /api/v1/system/coredump/raw`, cioè qualcuno deve accorgersene e andarlo a
prendere. Il firmware ora lo manda da solo sul broker; il go-client lo deve
ricomporre, salvare su disco e confermare, e solo allora il device cancella la
flash.

## Il protocollo (contratto con il firmware)

```
<prefix><dev>/coredump/<cid>/data/<n>   byte grezzi, pezzo n          QoS 1, retained
<prefix><dev>/coredump/<cid>/meta       JSON, pubblicato DOPO i pezzi  QoS 1, retained
<prefix><dev>/coredump/<cid>/ack        "ok", lo pubblica il go-client QoS 1, retained
```

- `<prefix>` = `topicPrefix` (`termostato/`), `<dev>` come in `SPEC-multi-device.md`.
- `<cid>` = CRC-32 zlib/IEEE dell'immagine **senza gli ultimi 4 byte**, **8 cifre
  esadecimali minuscole** (`fmt.Sprintf("%08x", crc32.ChecksumIEEE(img[:len(img)-4]))`).
  L'immagine in flash finisce con il CRC-32 del resto (little endian), quindi il
  CRC dell'immagine intera è sempre la costante `2144df1c`, uguale per ogni core
  dump: non può fare da id.
- `<n>` = indice decimale del pezzo, da 0 a `chunks-1`, senza zeri iniziali.
  Ogni pezzo è lungo `chunk_size` (1024) byte tranne l'ultimo.
- Il device manda un pezzo alla volta (il successivo dopo il PUBACK), poi `meta`.
  Ma i messaggi sono **retained**: alla (ri)connessione del go-client il broker li
  consegna tutti in un ordine qualsiasi. Il ricevitore non deve fare ipotesi
  sull'ordine né su `meta` come ultimo arrivato.
- **Reinvii**: senza ack il device ripubblica tutto ogni 12 h e a ogni avvio,
  sugli stessi topic (stesso `<cid>`). Il ricevitore deve essere idempotente.
- **Payload vuoto** su `data/<n>`, `meta` o `ack` = il device sta ripulendo i
  retained dopo l'ack: **ignorarlo** in silenzio (al più log Debug). Non è un
  errore e non deve toccare lo stato.
- Il device cancella la flash solo dopo aver visto l'ack e ripulito i retained.
  Se si riavvia a metà ripulitura il dump torna: il go-client lo ha già in
  tabella e risponde solo con l'ack (vedi sotto).

### `meta`

```json
{"device":"a1b2c3","id":"9f3c01aa","size":61476,"chunk_size":1024,"chunks":61,
 "crc32":"9f3c01aa","elf_sha256":"3be1c8d2","task":"control",
 "pc":"0x400d1234","backtrace":["0x400d1234","0x400d5678"],
 "backtrace_corrupted":false,"panic_reason":"LoadProhibited ..."}
```

- Sempre presenti: `device`, `id`, `size`, `chunk_size`, `chunks`, `crc32`
  (uguale a `id`).
- Possono mancare (il firmware non è riuscito a leggere il riassunto): da
  `elf_sha256` in giù. Mancanti → NULL in tabella, nessun errore.
- `elf_sha256` è **solo un prefisso** dello SHA-256 dell'ELF dell'applicazione:
  8 cifre esadecimali (`CONFIG_APP_RETRIEVE_LEN_ELF_SHA=9` nel firmware, 8 + NUL).
  Trattarlo come stringa opaca, non validarne la lunghezza.
- `device` e `id` nel JSON devono coincidere con quelli del topic; se non
  coincidono vince il topic, log Warn.

## Cosa deve fare il go-client

### 1. Sottoscrizioni

In `subscribeAll()` (quindi anche ad ogni riconnessione, via `OnConnectHandler`),
QoS 1:

```
<prefix>+/coredump/+/data/+
<prefix>+/coredump/+/meta
```

**Non** `termostato/#` né `<prefix>+/coredump/#`: la seconda riceverebbe anche
l'`ack` che pubblichiamo noi stessi.

L'helper esistente `subscribe(filter, qos, handler func(device, payload))`
perde il resto del topic; qui serve il topic intero. Una funzione pura in
`topics.go`, con test:

```go
// coredumpFromTopic("termostato/", "termostato/a1b2c3/coredump/9f3c01aa/data/12")
//   -> dev "a1b2c3", cid "9f3c01aa", kind "data", n 12, ok true
// ".../meta" -> kind "meta", n 0
func coredumpFromTopic(prefix, topic string) (dev, cid, kind string, n int, ok bool)
```

Rifiuta (log Warn, messaggio scartato): `dev` non valido con le stesse regole di
`deviceFromTopic`; `cid` che non sia esattamente 8 caratteri `[0-9a-f]`;
`n` non decimale, negativo, con zeri iniziali o ≥ 1024; foglie diverse da
`data/<n>` e `meta`; livelli in più.

### 2. Ricomposizione

Stato in memoria, protetto da mutex (paho può chiamare gli handler da goroutine
diverse se `OrderMatters` è false):

```go
type key struct{ dev, cid string }
type partial struct {
    chunks   map[int][]byte
    meta     *coredumpMeta   // nil finché non arriva
    first    time.Time       // primo pezzo visto, per la scadenza
}
```

A ogni messaggio non vuoto:

1. Se `(dev,cid)` è già nella tabella `coredumps`: **niente riscrittura**,
   si scarta il pezzo, si scarta lo stato parziale se c'è, e si ripubblica l'ack
   (al più una volta per `(dev,cid)` ogni pochi secondi, altrimenti un reinvio di
   60 pezzi produce 60 ack). Per non interrogare il DB a ogni pezzo, tenere in
   memoria l'insieme delle chiavi già salvate, caricato all'avvio e aggiornato
   dopo ogni inserimento.
2. Altrimenti si salva il pezzo (un duplicato sovrascrive: stesso contenuto) o
   il meta.
3. Completo quando c'è il meta e ci sono tutti i pezzi `0..chunks-1`. Allora:
   - concatenare in ordine; verificare `len == size` e
     il CRC dei primi `size - 4` byte `== crc32` del meta (== `cid`);
   - **sbagliato** → log Error con dev, cid, size attesa/ottenuta, CRC
     atteso/ottenuto; scartare lo stato parziale; **nessun ack**. Il device
     ripubblicherà fra 12 h o al prossimo avvio;
   - **giusto** → salvataggio (punto 3), poi ack.

Limiti, controllati appena arriva il meta (e per i pezzi, su `n`):

- `size` ≤ 1 MB, `chunks` ≤ 1024, `chunk_size` > 0,
  `chunks == ceil(size / chunk_size)`; ogni pezzo lungo ≤ `chunk_size`.
- Violati → log Error, stato parziale scartato, nessun ack.
- Stato parziale scade dopo **24 h** dal primo pezzo (un ticker ogni ora lo
  ripulisce, log Warn con quanti pezzi mancavano). Il device ripubblica ogni 12 h,
  quindi entro la scadenza c'è sempre almeno un reinvio completo.
- Massimo 16 `(dev,cid)` parziali contemporanei: oltre, si scarta il più vecchio.

### 3. Salvataggio

Nuova chiave in `config.yaml`:

```yaml
coredumpDir: /data/coredumps   # obbligatoria se si vuole il flusso coredump
elfDir: /data/elf              # opzionale, vedi punto 5
```

e un volume in `docker-compose.yml` per `mqttclient`
(`./coredumps:/data/coredumps`, creata con i permessi dell'utente del container).

File, con `<data>` = data UTC di ricezione `20261009T143012Z`:

```
<coredumpDir>/<dev>/<data>-<cid>-<elf_sha256>.elf    l'immagine
<coredumpDir>/<dev>/<data>-<cid>-<elf_sha256>.json   il meta così come arrivato
```

(`elf_sha256` mancante → `unknown`.) Scrivere su file temporaneo nella stessa
directory e poi `os.Rename`, così un crash a metà non lascia un `.elf` troncato.

Poi `InsertCoredump` (sotto), poi l'ack. Ordine importante: l'ack dice al device
"puoi cancellare la flash", quindi va pubblicato **solo dopo** che file e riga
sono scritti. Se il file è scritto ma l'INSERT fallisce → niente ack, log Error;
al reinvio si riscrive (nome con data diversa, accettabile).

### 4. Ack

```go
mc.client.Publish(prefix+dev+"/coredump/"+cid+"/ack", 1, true, "ok")
```

Attenzione: con `OrderMatters(true)` (default di paho) **non** chiamare
`token.Wait()` dentro un message handler, si blocca. Pubblicare senza `Wait`
e controllare l'errore in una goroutine, oppure fare tutto il lavoro di
completamento in una goroutine.

L'ack resta retained finché il device non lo ripulisce: è voluto, così un
device che si riconnette dopo giorni lo trova.

### 5. Opzionale: decodifica

Se `elfDir` è configurata ed esiste `<elfDir>/Termostato-<elf_sha256>*.elf`
(glob: `elf_sha256` è un prefisso), lanciare in una goroutine, con timeout 60 s:

```sh
esp-coredump info_corefile -c <file>.elf -t raw <Termostato-xxx.elf>
```

e salvare stdout+stderr in `<data>-<cid>-<elf_sha256>.txt` accanto all'ELF.
Errori → log Warn, mai blocco dell'ack. Richiede `esp-coredump`
(`pip install esp-coredump`) e un `xtensa-esp32-elf-gdb` nell'immagine: se
appesantisce troppo il Dockerfile, rimandare. Gli ELF dell'applicazione li
copia lì l'utente (`build/Termostato.elf` del firmware, rinominato col prefisso
SHA che `idf.py` stampa come `App version`/`ELF file SHA256`).

### 6. Database — migrazione `010_add_coredumps.sql` (Tern)

```sql
CREATE TABLE coredumps (
    "device"       TEXT NOT NULL,
    "id"           TEXT NOT NULL,              -- cid, CRC-32 hex
    "received"     TIMESTAMP NOT NULL DEFAULT now(),
    "size"         INTEGER NOT NULL,
    "elf_sha256"   TEXT,
    "task"         TEXT,
    "pc"           TEXT,
    "panic_reason" TEXT,
    "backtrace"    TEXT[],
    "path"         TEXT NOT NULL,              -- percorso del .elf
    PRIMARY KEY ("device", "id")
);

---- create above / drop below ----

DROP TABLE coredumps;
```

- Aggiornare anche `sql/schema/005_coredumps.sql`, `init_existing_db.sql` e
  l'elenco in `MIGRATIONS.md`, poi `make sqlc`.
- Query in `sql/queries/coredumps.sql`:
  - `InsertCoredump` — `INSERT ... ON CONFLICT ("device","id") DO NOTHING`
    (due reinvii ricomposti in parallelo non devono fallire);
  - `ListCoredumpKeys` — `SELECT device, id FROM coredumps`, per l'insieme in
    memoria del punto 2.
- Fa anche `touchDevice` come gli altri messaggi.

### 7. Broker: persistenza dei retained

`infra/mosquitto/config/mosquitto.conf` non ha `persistence`: al riavvio di
mosquitto tutti i retained (pezzi, meta, ack, ma anche `status` e `info`) si
perdono. Aggiungere:

```
persistence true
persistence_location /mosquitto/data/
```

La directory è già montata in `docker-compose.yml`
(`./infra/mosquitto/data/:/mosquitto/data`). Verificare che l'utente
`mosquitto` del container possa scriverci.

Senza persistenza il protocollo regge comunque (il device ripubblica ogni 12 h
finché non vede l'ack), ma un ack perso costringe a un reinvio inutile.

## Test

Unitari (senza broker, il ricompositore come struct con un'interfaccia per
"salva" e "pubblica ack"):

- `coredumpFromTopic`: casi validi `data/0`, `data/1023`, `meta`; rifiutati
  `data/01`, `data/-1`, `data/1024`, `data/x`, `ack`, cid di 7 o 9 caratteri,
  cid maiuscolo, livelli in più, device vuoto, prefisso diverso.
- Pezzi fuori ordine, meta per primo, meta in mezzo: completa una volta sola.
- Pezzo mancante: nessun salvataggio, nessun ack; scaduto dopo 24 h
  (orologio iniettabile).
- CRC sbagliato (un byte cambiato) e `size` sbagliata: log, niente ack, stato
  ripulito.
- Duplicati di un pezzo prima del completamento: nessun effetto.
- Riconsegna dopo il salvataggio (stesso `(dev,cid)`): nessuna riscrittura,
  ack ripubblicato, ack non ripetuto per ogni pezzo.
- Payload vuoti: ignorati, non creano stato parziale.
- Limiti: `size` > 1 MB, `chunks` > 1024, `chunks` incoerente con `size`.

Prova manuale con mosquitto:

```sh
P=termostato/test01/coredump
head -c 5000 /dev/urandom > /tmp/cd.bin
CID=$(python3 -c 'import zlib,sys;print("%08x"%zlib.crc32(open("/tmp/cd.bin","rb").read()))')
split -b 1024 -d -a 4 /tmp/cd.bin /tmp/cd.part.
i=0; for f in /tmp/cd.part.*; do
  mosquitto_pub -p 1885 -q 1 -r -t $P/$CID/data/$i -f $f; i=$((i+1)); done
mosquitto_pub -p 1885 -q 1 -r -t $P/$CID/meta -m \
  "{\"device\":\"test01\",\"id\":\"$CID\",\"size\":5000,\"chunk_size\":1024,\"chunks\":$i,\"crc32\":\"$CID\"}"
mosquitto_sub -p 1885 -t $P/$CID/ack -C 1 -W 10     # -> ok
cmp /tmp/cd.bin coredumps/test01/*-$CID-unknown.elf
```

Ripetere la stessa sequenza: nessun nuovo file, l'ack torna. Ripulire con
`mosquitto_pub -p 1885 -r -n -t $P/$CID/...` per ogni topic (il go-client deve
ignorare questi payload vuoti senza errori).

## Fuori da questa specifica

- Una pagina Grafana o un endpoint per elencare i core dump: per ora basta
  `SELECT * FROM coredumps ORDER BY received DESC`.
- Notifiche (mail, Telegram) all'arrivo di un dump.
