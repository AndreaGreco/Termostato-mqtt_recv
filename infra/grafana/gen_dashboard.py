#!/usr/bin/env python3
"""Genera termostato.json, la dashboard Grafana del termostato.

    python3 gen_dashboard.py > termostato.json

Poi in Grafana: Dashboards -> New -> Import, incolla il file. Lo uid e' quello
della dashboard esistente, quindi la sovrascrive.

Regole seguite:
- un solo asse per pannello: grandezze diverse (gradi, %, l/min, byte) stanno
  in pannelli diversi;
- ogni serie ha sempre lo stesso colore, in tutti i pannelli;
- setpoint e limiti tratteggiati, misure a linea piena;
- gli stati acceso/spento sono una state timeline, non linee 0/1;
- niente pannelli su campi che questa caldaia non fornisce (contatori del
  bruciatore, fumi, potenza: sempre 0 da quando esiste la tabella).
"""

import json

DS = {"type": "grafana-postgresql-datasource", "uid": "dfd5otd4lezggd"}
DEV = "\"device\" = ${device:sqlstring}"

# Colore fisso per entita'
C = {
    "ambiente": "blue",
    "esterna": "yellow",
    "setpoint": "orange",
    "mandata": "red",
    "ritorno": "blue",
    "limite": "#8e8e8e",
    "acs": "purple",
    "portata": "blue",
    "sens_a": "blue",
    "sens_b": "purple",
    "sens_hot": "red",
    "heap": "green",
    "blocco": "blue",
}

_next_id = [1]


def pid():
    _next_id[0] += 1
    return _next_id[0]


def target(sql, fmt="time_series", ref="A"):
    return {"datasource": DS, "editorMode": "code", "format": fmt,
            "rawQuery": True, "rawSql": sql, "refId": ref}


def color(c):
    return {"id": "color", "value": {"mode": "fixed", "fixedColor": c}}


def series(name, c, dashed=False, width=2, step=False):
    props = [color(c), {"id": "custom.lineWidth", "value": width}]
    if dashed:
        props.append({"id": "custom.lineStyle",
                      "value": {"fill": "dash", "dash": [8, 6]}})
    if step:
        props.append({"id": "custom.lineInterpolation", "value": "stepAfter"})
    return {"matcher": {"id": "byName", "options": name}, "properties": props}


def row(title, y):
    return {"type": "row", "title": title, "collapsed": False, "panels": [],
            "id": pid(), "gridPos": {"h": 1, "w": 24, "x": 0, "y": y}}


def timeseries(title, grid, sql, unit, overrides, interval, decimals=1,
               minv=None, maxv=None, bars=False, description="", extra=()):
    """extra: altre query sullo stesso asse, ad esempio da un'altra tabella"""
    custom = {
        "drawStyle": "bars" if bars else "line",
        "lineWidth": 2,
        "lineInterpolation": "linear",
        "fillOpacity": 60 if bars else 0,
        "showPoints": "never",
        "spanNulls": False,
        "axisPlacement": "auto",
        "axisSoftMin": minv,
        "axisSoftMax": maxv,
        "thresholdsStyle": {"mode": "off"},
    }
    return {
        "type": "timeseries", "title": title, "id": pid(), "gridPos": grid,
        "datasource": DS, "interval": interval, "description": description,
        "fieldConfig": {
            "defaults": {"unit": unit, "decimals": decimals,
                         "color": {"mode": "palette-classic"},
                         "custom": custom},
            "overrides": overrides,
        },
        "options": {
            "legend": {"displayMode": "list", "placement": "bottom",
                       "showLegend": True, "calcs": []},
            "tooltip": {"mode": "multi", "sort": "none"},
        },
        "targets": [target(sql)] + [target(q, ref=chr(ord("B") + i))
                                    for i, q in enumerate(extra)],
    }


def stat(title, grid, sql, unit, decimals=1, mappings=None, description=""):
    return {
        "type": "stat", "title": title, "id": pid(), "gridPos": grid,
        "datasource": DS, "description": description,
        "fieldConfig": {
            "defaults": {
                "unit": unit, "decimals": decimals,
                "color": {"mode": "fixed", "fixedColor": "text"},
                "mappings": mappings or [],
                "noValue": "—",
            },
            "overrides": [],
        },
        "options": {
            "colorMode": "value", "graphMode": "none", "justifyMode": "center",
            "textMode": "value_and_name", "orientation": "horizontal",
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "",
                              "values": False},
        },
        "targets": [target(sql, fmt="table")],
    }


# Acceso/spento: testo oltre al colore, mai il solo colore
ON_OFF = [{"type": "value", "options": {
    "0": {"text": "spento", "color": "transparent", "index": 0},
    "1": {"text": "acceso", "color": "orange", "index": 1}}}]

panels = []
y = 0

# --- Adesso ---------------------------------------------------------------
panels.append(row("Adesso", y)); y += 1

def last(table, col, name):
    """Ultimo valore, indipendente dall'intervallo di tempo scelto"""
    return ("SELECT " + col + " AS \"" + name + "\" FROM " + table +
            " WHERE " + DEV + " ORDER BY \"time\" DESC LIMIT 1")

panels.append(stat("Ambiente", {"h": 4, "w": 3, "x": 0, "y": y},
                   last("measure", "\"AmbTemp\"", "Ambiente"),
                   "celsius"))
panels.append(stat("Setpoint", {"h": 4, "w": 3, "x": 3, "y": y},
                   last("measure", "\"AmbientSetpoint\"", "Setpoint"),
                   "celsius"))
panels.append(stat("Esterna", {"h": 4, "w": 3, "x": 6, "y": y},
                   last("opentherm", "\"weatherTemp\"", "Esterna"),
                   "celsius",
                   description="Dal meteo (Open-Meteo), la stessa che usa il "
                               "modello termico. Vuota se il termostato non "
                               "ha una previsione."))
panels.append(stat("Mandata", {"h": 4, "w": 3, "x": 9, "y": y},
                   last("opentherm", "\"heatingWaterTempRead\"", "Mandata"),
                   "celsius"))
panels.append(stat("Modulazione", {"h": 4, "w": 3, "x": 12, "y": y},
                   last("opentherm", "\"modulationLvl\"", "Modulazione"),
                   "percent", decimals=0))
panels.append(stat(
    "Caldaia", {"h": 4, "w": 6, "x": 15, "y": y},
    "SELECT \"heatingOn\"::int AS \"Riscaldamento\", \"flameOn\"::int AS \"Fiamma\", "
    "\"dhwOn\"::int AS \"ACS\" FROM opentherm WHERE " + DEV +
    " ORDER BY \"time\" DESC LIMIT 1",
    "none", decimals=0, mappings=ON_OFF))
panels.append(stat(
    "Portata ACS", {"h": 4, "w": 3, "x": 21, "y": y},
    last("opentherm", "\"dhwFlowRate\"", "Portata"),
    "flowlpm", mappings=[{"type": "special", "options": {
        "match": "null", "result": {"text": "chiusa", "index": 0}}}],
    description="Id OpenTherm 19. La caldaia risponde Data-Invalid a "
                "rubinetti chiusi: qui diventa 'chiusa'."))
y += 4

# --- Ambiente -------------------------------------------------------------
panels.append(row("Ambiente", y)); y += 1

panels.append(timeseries(
    "Temperatura ambiente", {"h": 9, "w": 16, "x": 0, "y": y},
    "SELECT $__timeGroupAlias(\"time\", $__interval), "
    "avg(\"AmbTemp\") AS \"Ambiente\", avg(\"AmbientSetpoint\") AS \"Setpoint\" "
    "FROM measure WHERE $__timeFilter(\"time\") AND " + DEV +
    " GROUP BY 1 ORDER BY 1",
    "celsius",
    [series("Ambiente", C["ambiente"]),
     series("Setpoint", C["setpoint"], dashed=True, step=True),
     series("Esterna (meteo)", C["esterna"], width=1)],
    interval="30s", decimals=2,
    description="La temperatura su cui regola il termostato (remota se il "
                "sensore remoto e' fresco, altrimenti la media degli interni). "
                "Esterna: dal meteo, la stessa del modello termico.",
    extra=["SELECT $__timeGroupAlias(\"time\", $__interval), "
           "avg(\"weatherTemp\") AS \"Esterna (meteo)\" "
           "FROM opentherm WHERE $__timeFilter(\"time\") AND " + DEV +
           " GROUP BY 1 ORDER BY 1"]))

panels.append(timeseries(
    "Sensori interni", {"h": 9, "w": 8, "x": 16, "y": y},
    "SELECT $__timeGroupAlias(\"time\", $__interval), "
    "avg(\"TempSensA\") AS \"Sensore A\", avg(\"TempSensB\") AS \"Sensore B\", "
    "avg(\"TempHot\") AS \"Piu' caldo\" "
    "FROM measure WHERE $__timeFilter(\"time\") AND " + DEV +
    " GROUP BY 1 ORDER BY 1",
    "celsius",
    [series("Sensore A", C["sens_a"], width=1),
     series("Sensore B", C["sens_b"], width=1),
     series("Piu' caldo", C["sens_hot"], width=1)],
    interval="30s", decimals=2,
    description="Sensori nella scatola. Con il sensore remoto attivo la "
                "scatola si scalda (niente light sleep): qui si vede."))
y += 9

# --- Caldaia --------------------------------------------------------------
panels.append(row("Caldaia", y)); y += 1

panels.append(timeseries(
    "Mandata e ritorno", {"h": 9, "w": 16, "x": 0, "y": y},
    "SELECT $__timeGroupAlias(\"time\", $__interval), "
    "avg(\"heatingWaterTempRead\") AS \"Mandata\", "
    "avg(\"returnWaterTemp\") AS \"Ritorno\", "
    # setpoint 0 = riscaldamento spento, non un comando a 0 gradi
    "avg(NULLIF(\"heatingSetpointRead\", 0)) AS \"Setpoint mandata\", "
    "avg(\"maxChSetpoint\") AS \"Massimo caldaia\" "
    "FROM opentherm WHERE $__timeFilter(\"time\") AND " + DEV +
    " GROUP BY 1 ORDER BY 1",
    "celsius",
    [series("Mandata", C["mandata"]),
     series("Ritorno", C["ritorno"]),
     series("Setpoint mandata", C["setpoint"], dashed=True, step=True),
     series("Massimo caldaia", C["limite"], dashed=True, width=1, step=True)],
    interval="10s",
    description="Il setpoint e' cio' che la caldaia rilegge del comando; "
                "manca quando il riscaldamento e' spento. Massimo caldaia = "
                "id OpenTherm 57, il firmware non manda mai oltre."))

panels.append(timeseries(
    "Modulazione", {"h": 9, "w": 8, "x": 16, "y": y},
    "SELECT $__timeGroupAlias(\"time\", $__interval), "
    "avg(\"modulationLvl\") AS \"Modulazione\" "
    "FROM opentherm WHERE $__timeFilter(\"time\") AND " + DEV +
    " GROUP BY 1 ORDER BY 1",
    "percent",
    [series("Modulazione", C["mandata"])],
    interval="10s", decimals=0, minv=0, maxv=100))
y += 9

panels.append({
    "type": "state-timeline", "title": "Stato", "id": pid(),
    "gridPos": {"h": 5, "w": 24, "x": 0, "y": y}, "datasource": DS,
    "interval": "10s",
    "fieldConfig": {
        "defaults": {
            "color": {"mode": "thresholds"},
            "mappings": [{"type": "value", "options": {
                "0": {"text": "spento", "color": "transparent", "index": 0},
                "1": {"text": "acceso", "index": 1}}}],
            "thresholds": {"mode": "absolute", "steps": [
                {"color": "transparent", "value": None},
                {"color": "orange", "value": 1}]},
            "custom": {"fillOpacity": 80, "lineWidth": 0},
        },
        "overrides": [
            {"matcher": {"id": "byName", "options": "Fiamma"},
             "properties": [{"id": "thresholds", "value": {
                 "mode": "absolute", "steps": [
                     {"color": "transparent", "value": None},
                     {"color": "red", "value": 1}]}}]},
            {"matcher": {"id": "byName", "options": "ACS"},
             "properties": [{"id": "thresholds", "value": {
                 "mode": "absolute", "steps": [
                     {"color": "transparent", "value": None},
                     {"color": "purple", "value": 1}]}}]},
        ],
    },
    "options": {"showValue": "never", "mergeValues": True, "rowHeight": 0.8,
                "alignValue": "left",
                "legend": {"showLegend": False},
                "tooltip": {"mode": "multi"}},
    "targets": [target(
        "SELECT $__timeGroupAlias(\"time\", $__interval), "
        "bool_or(\"heatingOn\")::int AS \"Riscaldamento\", "
        "bool_or(\"flameOn\")::int AS \"Fiamma\", "
        "bool_or(\"dhwOn\")::int AS \"ACS\" "
        "FROM opentherm WHERE $__timeFilter(\"time\") AND " + DEV +
        " GROUP BY 1 ORDER BY 1")],
})
y += 5

# --- Acqua calda ----------------------------------------------------------
panels.append(row("Acqua calda sanitaria", y)); y += 1

panels.append(timeseries(
    "Temperatura ACS", {"h": 8, "w": 16, "x": 0, "y": y},
    "SELECT $__timeGroupAlias(\"time\", $__interval), "
    "avg(\"dhwTemperature\") AS \"ACS\", avg(\"dhwSetpoint\") AS \"Setpoint ACS\", "
    "avg(\"dhwSetpointMin\") AS \"Minimo caldaia\", "
    "avg(\"dhwSetpointMax\") AS \"Massimo caldaia\" "
    "FROM opentherm WHERE $__timeFilter(\"time\") AND " + DEV +
    " GROUP BY 1 ORDER BY 1",
    "celsius",
    [series("ACS", C["acs"]),
     series("Setpoint ACS", C["setpoint"], dashed=True, step=True),
     series("Minimo caldaia", C["limite"], dashed=True, width=1, step=True),
     series("Massimo caldaia", C["limite"], dashed=True, width=1, step=True)],
    interval="10s",
    description="Limiti = id OpenTherm 48: il firmware stringe il setpoint "
                "ACS fra questi due."))

panels.append(timeseries(
    "Portata ACS", {"h": 8, "w": 8, "x": 16, "y": y},
    "SELECT $__timeGroupAlias(\"time\", $__interval), "
    "max(\"dhwFlowRate\") AS \"Portata\" "
    "FROM opentherm WHERE $__timeFilter(\"time\") AND " + DEV +
    " GROUP BY 1 ORDER BY 1",
    "flowlpm", [series("Portata", C["portata"])],
    interval="10s", minv=0, bars=True,
    description="Id OpenTherm 19: vuoto a rubinetti chiusi. Distingue un "
                "prelievo di acqua calda dal riscaldamento."))
y += 8

# --- Sistema --------------------------------------------------------------
panels.append(row("Sistema", y)); y += 1

heap = timeseries(
    "Heap", {"h": 8, "w": 12, "x": 0, "y": y},
    "SELECT $__timeGroupAlias(\"time\", $__interval), "
    "min(heap_free) AS \"Libero\", min(heap_largest_block) AS \"Blocco piu' grande\" "
    "FROM measure WHERE $__timeFilter(\"time\") AND " + DEV +
    " GROUP BY 1 ORDER BY 1",
    "bytes",
    [series("Libero", C["heap"]), series("Blocco piu' grande", C["blocco"])],
    interval="30s", decimals=0, minv=0,
    description="Minimo per intervallo, non media: conta il punto piu' basso. "
                "La linea a 16 KB e' la soglia sotto cui il meteo salta il giro.")
heap["fieldConfig"]["defaults"]["thresholds"] = {
    "mode": "absolute", "steps": [{"color": "transparent", "value": None},
                                  {"color": "#8e8e8e", "value": 16384}]}
heap["fieldConfig"]["defaults"]["custom"]["thresholdsStyle"] = {"mode": "dashed"}
panels.append(heap)

panels.append({
    "type": "table", "title": "Dispositivo", "id": pid(),
    "gridPos": {"h": 4, "w": 12, "x": 12, "y": y}, "datasource": DS,
    "fieldConfig": {"defaults": {}, "overrides": []},
    "options": {"showHeader": True, "cellHeight": "sm"},
    "targets": [target(
        "SELECT fw_version AS \"Firmware\", partition AS \"Partizione\", "
        "ip AS \"IP\", CASE WHEN online THEN 'online' ELSE 'offline' END AS \"Stato\", "
        "last_seen AS \"Visto\" FROM devices WHERE id = ${device:sqlstring}",
        fmt="table")],
})

panels.append(stat(
    "Riavvii per panic", {"h": 4, "w": 12, "x": 12, "y": y + 4},
    "SELECT boot_count AS \"Riavvii per panic\" FROM measure WHERE " + DEV +
    " ORDER BY \"time\" DESC LIMIT 1", "none", decimals=0))
y += 8

panels.append({
    "type": "table", "title": "Core dump", "id": pid(),
    "gridPos": {"h": 6, "w": 24, "x": 0, "y": y}, "datasource": DS,
    "fieldConfig": {"defaults": {}, "overrides": []},
    "options": {"showHeader": True, "cellHeight": "sm",
                "sortBy": [{"displayName": "Ricevuto", "desc": True}]},
    "targets": [target(
        "SELECT received AS \"Ricevuto\", task AS \"Task\", "
        "COALESCE(panic_reason, '') AS \"Motivo\", pc AS \"PC\", "
        "elf_sha256 AS \"ELF\", id AS \"Id\" FROM coredumps "
        "WHERE device = ${device:sqlstring} ORDER BY received DESC LIMIT 20",
        fmt="table")],
})
y += 6

panels.append({
    "type": "logs", "title": "Log", "id": pid(),
    "gridPos": {"h": 10, "w": 24, "x": 0, "y": y}, "datasource": DS,
    "options": {"showTime": True, "wrapLogMessage": True,
                "sortOrder": "Descending", "enableLogDetails": False},
    "targets": [target(
        "SELECT \"time\", task || ': ' || msg AS \"msg\" FROM logs "
        "WHERE $__timeFilter(\"time\") AND " + DEV +
        " ORDER BY \"time\" DESC LIMIT 500", fmt="table")],
})

dashboard = {
    "id": None,
    "uid": "adjfs52",
    "title": "Termostato",
    "tags": ["termostato"],
    "timezone": "browser",
    "editable": True,
    "graphTooltip": 1,          # crosshair condiviso fra i pannelli
    "refresh": "30s",
    "schemaVersion": 42,
    "time": {"from": "now-6h", "to": "now"},
    "fiscalYearStartMonth": 0,
    "links": [],
    "annotations": {"list": [{
        "builtIn": 1, "datasource": {"type": "grafana", "uid": "-- Grafana --"},
        "enable": True, "hide": True, "iconColor": "rgba(0, 211, 255, 1)",
        "name": "Annotations & Alerts", "type": "dashboard"}]},
    "templating": {"list": [{
        "name": "device", "label": "Termostato", "type": "query",
        "datasource": DS, "refresh": 1,
        "definition": "SELECT id AS __value, COALESCE(name, id) AS __text FROM devices ORDER BY id",
        "query": "SELECT id AS __value, COALESCE(name, id) AS __text FROM devices ORDER BY id",
        "current": {"text": "2e79cc", "value": "2e79cc"},
    }]},
    "panels": panels,
}

print(json.dumps(dashboard, indent=2, ensure_ascii=False))
