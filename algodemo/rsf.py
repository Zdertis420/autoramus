#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
rsf.py — чтение и запись файлов Ramus (*.rsf) в обход самой программы Ramus.

Формат (проверено по исходникам Ramus, com.ramussoft.core.impl.FileIEngineImpl,
TableToXML / XMLToTable):

  *.rsf                = обычный ZIP
  data/*.xml           = дампы таблиц встроенной SQL-БД (<table><fields/><data/></table>)
  data/<Plugin>/*.xml  = таблицы значений атрибутов ("персистенты")
  data/application_metadata.xml, data/sequences.xml = java.util.Properties (XML DTD)
  properties/*, user/* = произвольные потоки (streams), перечислены в data/streams.xml

Правила загрузки в Ramus (важно для записи):
  * поля строк сопоставляются по ИМЕНИ (<field id=.. name=.. type=..>), а не по порядку;
  * отсутствующий <f> = NULL, при этом *_branch_id по умолчанию 0,
    а removed_branch_id по умолчанию 2147483647;
  * BLOB кодируется как hex от (байт + 128);
  * TIMESTAMP пишется в формате DateFormat.SHORT/SHORT Locale.ENGLISH: "9/4/26 10:23 AM";
  * data/persistents.xml и data/persistent_fields.xml при открытии НЕ читаются
    (схема пересобирается из аннотаций плагинов), но лучше их сохранять как есть;
  * порядок entry в ZIP значения не имеет.
"""

from __future__ import annotations

import io
import zipfile
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from typing import Dict, List, Optional

ALIVE = "2147483647"          # removed_branch_id живой строки
DEAD = "0"                    # removed_branch_id удалённой строки


# --------------------------------------------------------------------------- #
#  Низкий уровень: таблицы
# --------------------------------------------------------------------------- #

@dataclass
class Table:
    """Одна таблица data/**/*.xml."""
    path: str                       # путь внутри zip
    name: str                       # generate-from-table
    prefix: str                     # "ramus_"
    fields: List[dict]              # [{'id','name','type'}, ...]
    rows: List[Dict[str, Optional[str]]]

    def col_id(self, name: str) -> str:
        for f in self.fields:
            if f["name"].upper() == name.upper():
                return f["id"]
        raise KeyError(f"{self.name}: нет поля {name}")

    def add(self, **values) -> Dict[str, Optional[str]]:
        """Добавить строку. Ключи — имена полей (регистр не важен)."""
        norm = {f["name"]: None for f in self.fields}
        upper = {f["name"].upper(): f["name"] for f in self.fields}
        for k, v in values.items():
            if k.upper() not in upper:
                raise KeyError(f"{self.name}: нет поля {k}")
            norm[upper[k.upper()]] = None if v is None else str(v)
        self.rows.append(norm)
        return norm

    def select(self, **where) -> List[Dict[str, Optional[str]]]:
        res = []
        for r in self.rows:
            ok = True
            for k, v in where.items():
                key = next((f["name"] for f in self.fields
                            if f["name"].upper() == k.upper()), None)
                if key is None or (r.get(key) != (None if v is None else str(v))):
                    ok = False
                    break
            if ok:
                res.append(r)
        return res

    def to_xml(self, generate_time: str) -> bytes:
        out = io.StringIO()
        out.write('<?xml version="1.0" encoding="UTF-8"?>')
        out.write(f'<table generate-from-table="{_esc(self.name)}" '
                  f'generate-time="{_esc(generate_time)}" prefix="{_esc(self.prefix)}">')
        out.write("<fields>")
        for f in self.fields:
            out.write(f'<field id="{f["id"]}" name="{_esc(f["name"])}" type="{_esc(f["type"])}"/>')
        out.write("</fields><data>")
        for row in self.rows:
            out.write("<row>")
            for f in self.fields:
                v = row.get(f["name"])
                if v is None:
                    continue                      # NULL = поле просто отсутствует
                if v == "":
                    out.write(f'<f id="{f["id"]}"/>')
                else:
                    out.write(f'<f id="{f["id"]}">{_esc(v)}</f>')
            out.write("</row>")
        out.write("</data></table>")
        return out.getvalue().encode("utf-8")


def _esc(s: str) -> str:
    return (s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
             .replace('"', "&quot;"))


def blob_to_bytes(hexstr: str) -> bytes:
    """BLOB из XML -> реальные байты (Ramus хранит hex от byte+128)."""
    return bytes((int(hexstr[i:i + 2], 16) - 128) & 0xFF for i in range(0, len(hexstr), 2))


def bytes_to_blob(data: bytes) -> str:
    return "".join("%02X" % ((b + 128) & 0xFF) for b in data)


# --------------------------------------------------------------------------- #
#  Файл целиком
# --------------------------------------------------------------------------- #

class Rsf:
    def __init__(self, path: str):
        self.src_path = path
        self.tables: Dict[str, Table] = {}
        self.raw: Dict[str, bytes] = {}          # всё, что не таблица
        self.order: List[str] = []
        self.generate_time = "Sat Sep 05 00:00:00 MSK 2026"

        with zipfile.ZipFile(path) as z:
            for info in z.infolist():
                data = z.read(info.filename)
                self.order.append(info.filename)
                t = _parse_table(info.filename, data)
                if t is None:
                    self.raw[info.filename] = data
                else:
                    self.tables[info.filename] = t
                    self.generate_time = t_time(data) or self.generate_time

    # ---------- доступ ----------
    def table(self, name: str) -> Table:
        """По короткому имени: 'elements', 'IDEF0/attribute_rectangles', ..."""
        for p, t in self.tables.items():
            if p == f"data/{name}.xml" or t.name == name:
                return t
        raise KeyError(name)

    def next_id(self, table: str, column: str) -> int:
        t = self.table(table)
        ids = [int(r[column]) for r in t.rows if r.get(column) not in (None, "")]
        return (max(ids) + 1) if ids else 1

    # ---------- запись ----------
    def save(self, path: str) -> None:
        with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
            for name in self.order:
                if name in self.tables:
                    z.writestr(name, self.tables[name].to_xml(self.generate_time))
                else:
                    z.writestr(name, self.raw[name])


def _parse_table(path: str, data: bytes) -> Optional[Table]:
    if not path.startswith("data/") or not path.endswith(".xml"):
        return None
    try:
        root = ET.fromstring(data)
    except ET.ParseError:
        return None
    if root.tag != "table":
        return None                                # application_metadata / sequences
    fields = [{"id": f.get("id"), "name": f.get("name"), "type": f.get("type")}
              for f in root.find("fields")]
    by_id = {f["id"]: f["name"] for f in fields}
    rows = []
    for row in root.find("data"):
        r = {f["name"]: None for f in fields}
        for f in row:
            r[by_id[f.get("id")]] = f.text if f.text is not None else ""
        rows.append(r)
    return Table(path, root.get("generate-from-table"), root.get("prefix", "ramus_"),
                 fields, rows)


def t_time(data: bytes) -> Optional[str]:
    try:
        return ET.fromstring(data).get("generate-time")
    except Exception:
        return None


# --------------------------------------------------------------------------- #
#  Модель IDEF0 поверх таблиц
# --------------------------------------------------------------------------- #

# типы функциональных блоков (com.ramussoft.pb.Function)
TYPE_PROCESS_KOMPLEX, TYPE_PROCESS, TYPE_PROCESS_PART = 0, 1, 2
TYPE_OPERATION, TYPE_ACTION = 3, 4
TYPE_EXTERNAL_REFERENCE, TYPE_DATA_STORE, TYPE_DFDS_ROLE = 1001, 1002, 1003

# стороны блока (com.ramussoft.pb.idef.visual.MovingPanel)
SIDE_RIGHT, SIDE_BOTTOM, SIDE_LEFT, SIDE_TOP = 0, 1, 2, 3
# для IDEF0: LEFT=вход, TOP=управление, RIGHT=выход, BOTTOM=механизм


class Idef0:
    """Удобная обёртка: функции, потоки, стрелки одной модели."""

    def __init__(self, rsf: Rsf):
        self.rsf = rsf
        self.elements = rsf.table("elements")
        self.qualifiers = rsf.table("qualifiers")
        self.texts = rsf.table("attribute_texts")
        self.hier = rsf.table("attribute_hierarchicals")
        self.longs = rsf.table("attribute_longs")
        self.other = rsf.table("attribute_other_elements")
        self.rect = rsf.table("attribute_rectangles")
        self.ftype = rsf.table("attribute_function_types")
        self.status = rsf.table("attribute_statuses")
        self.font = rsf.table("attribute_fonts")
        self.color = rsf.table("attribute_colors")
        self.decomp = rsf.table("attribute_decomposition_types")
        self.borders = rsf.table("attribute_sector_borders")
        self.points = rsf.table("attribute_sector_points")

        # системные идентификаторы атрибутов ищем по имени, а не хардкодим
        self.attr = {r["ATTRIBUTE_NAME"]: r["ATTRIBUTE_ID"]
                     for r in rsf.table("attributes").rows}
        self.qual = {r["QUALIFIER_NAME"]: r["QUALIFIER_ID"]
                     for r in self.qualifiers.rows}

        # квалификатор работ модели: элемент F_BASE_FUNCTIONS -> F_BASE_FUNCTION_QUALIFIER_ID
        self.model_element = None
        self.func_qualifier = None
        base_q = self.qual["F_BASE_FUNCTIONS"]
        a44 = self.attr["F_BASE_FUNCTION_QUALIFIER_ID"]
        for el in self.elements.select(QUALIFIER_ID=base_q, REMOVED_BRANCH_ID=ALIVE):
            for r in self.longs.select(ATTRIBUTE_ID=a44, ELEMENT_ID=el["ELEMENT_ID"]):
                if r["VALUE"] != base_q:               # не служебная "пустая" модель
                    self.model_element = el["ELEMENT_ID"]
                    self.func_qualifier = r["VALUE"]
        self.name_attr = None
        if self.func_qualifier:
            q = self.qualifiers.select(QUALIFIER_ID=self.func_qualifier)[0]
            self.name_attr = q["ATTRIBUTE_FOR_NAME"]   # обычно пользовательский "Название"

    # ---------- чтение ----------
    def _text(self, element_id: str, attribute_id: str) -> Optional[str]:
        rows = self.texts.select(ELEMENT_ID=element_id, ATTRIBUTE_ID=attribute_id)
        return rows[0]["VALUE"] if rows else None

    def _set_text(self, element_id: str, attribute_id: str, value: str) -> None:
        rows = self.texts.select(ELEMENT_ID=element_id, ATTRIBUTE_ID=attribute_id)
        if rows:
            rows[0]["VALUE"] = value
        else:
            self.texts.add(ATTRIBUTE_ID=attribute_id, ELEMENT_ID=element_id,
                           VALUE=value, VALUE_BRANCH_ID=0)

    def functions(self) -> List[dict]:
        res = []
        for el in self.elements.select(QUALIFIER_ID=self.func_qualifier,
                                       REMOVED_BRANCH_ID=ALIVE):
            eid = el["ELEMENT_ID"]
            h = self.hier.select(ELEMENT_ID=eid, ATTRIBUTE_ID=self.attr["HierarchicalAttribute"])
            rc = self.rect.select(ELEMENT_ID=eid)
            ft = self.ftype.select(ELEMENT_ID=eid)
            res.append({
                "id": eid,
                "name": self._text(eid, self.name_attr),
                "parent": h[0]["PARENT_ELEMENT_ID"] if h else None,
                "previous": h[0]["PREVIOUS_ELEMENT_ID"] if h else None,
                "bounds": (float(rc[0]["X"]), float(rc[0]["Y"]),
                           float(rc[0]["WIDTH"]), float(rc[0]["HEIGHT"])) if rc else None,
                "type": int(ft[0]["TYPE"]) if ft else None,
            })
        return sorted(res, key=lambda f: int(f["id"]))

    def streams(self) -> List[dict]:
        a = self.attr["F_STREAM_NAME"]
        res = []
        for el in self.elements.select(QUALIFIER_ID=self.qual["F_STREAMS"],
                                       REMOVED_BRANCH_ID=ALIVE):
            res.append({"id": el["ELEMENT_ID"], "name": self._text(el["ELEMENT_ID"], a)})
        return sorted(res, key=lambda s: int(s["id"]))

    def sectors(self) -> List[dict]:
        af, as_ = self.attr["F_FUNCTION_SECTOR"], self.attr["F_SECTOR_STREAM"]
        st, en = self.attr["F_SECTOR_BORDER_START"], self.attr["F_SECTOR_BORDER_END"]
        res = []
        for el in self.elements.select(QUALIFIER_ID=self.qual["F_SECTORS"],
                                       REMOVED_BRANCH_ID=ALIVE):
            eid = el["ELEMENT_ID"]
            f = self.other.select(ELEMENT_ID=eid, ATTRIBUTE_ID=af)
            s = self.other.select(ELEMENT_ID=eid, ATTRIBUTE_ID=as_)
            b1 = self.borders.select(ELEMENT_ID=eid, ATTRIBUTE_ID=st)
            b2 = self.borders.select(ELEMENT_ID=eid, ATTRIBUTE_ID=en)
            res.append({
                "id": eid,
                "diagram_function": f[0]["OTHER_ELEMENT"] if f else None,
                "stream": s[0]["OTHER_ELEMENT"] if s else None,
                "start": b1[0] if b1 else None,
                "end": b2[0] if b2 else None,
                "points": self.points.select(ELEMENT_ID=eid),
            })
        return sorted(res, key=lambda s: int(s["id"]))

    # ---------- изменение ----------
    def rename_function(self, element_id, new_name: str) -> None:
        self._set_text(str(element_id), self.name_attr, new_name)

    def rename_stream(self, element_id, new_name: str) -> None:
        self._set_text(str(element_id), self.attr["F_STREAM_NAME"], new_name)

    def set_bounds(self, element_id, x, y, w, h) -> None:
        rows = self.rect.select(ELEMENT_ID=str(element_id))
        if not rows:
            raise KeyError(element_id)
        rows[0].update(X=str(float(x)), Y=str(float(y)),
                       WIDTH=str(float(w)), HEIGHT=str(float(h)))

    def add_function(self, parent_id, name: str, x=100.0, y=100.0, w=138.0, h=72.0,
                     ftype: int = TYPE_PROCESS) -> str:
        """Добавить работу в декомпозицию parent_id. Возвращает id нового элемента."""
        eid = str(self.rsf.next_id("elements", "ELEMENT_ID"))
        self.elements.add(ELEMENT_ID=eid, ELEMENT_NAME="",
                          QUALIFIER_ID=self.func_qualifier,
                          CREATED_BRANCH_ID=0, REMOVED_BRANCH_ID=ALIVE)

        # последний ребёнок parent -> PREVIOUS_ELEMENT_ID (список односвязный)
        prev = "-1"
        children = [r for r in self.hier.rows
                    if r["PARENT_ELEMENT_ID"] == str(parent_id)]
        used_prev = {r["PREVIOUS_ELEMENT_ID"] for r in children}
        for c in children:
            if c["ELEMENT_ID"] not in used_prev:
                prev = c["ELEMENT_ID"]
        self.hier.add(ATTRIBUTE_ID=self.attr["HierarchicalAttribute"], ELEMENT_ID=eid,
                      ICON_ID=-1, PARENT_ELEMENT_ID=str(parent_id),
                      PREVIOUS_ELEMENT_ID=prev, VALUE_BRANCH_ID=0)

        self._set_text(eid, self.name_attr, name)
        self.rect.add(ATTRIBUTE_ID=self.attr["F_BOUNDS"], ELEMENT_ID=eid,
                      X=float(x), Y=float(y), WIDTH=float(w), HEIGHT=float(h),
                      VALUE_BRANCH_ID=0)
        self.ftype.add(ATTRIBUTE_ID=self.attr["F_TYPE"], ELEMENT_ID=eid,
                       TYPE=ftype, VALUE_BRANCH_ID=0)
        self.status.add(ATTRIBUTE_ID=self.attr["F_STATUS"], ELEMENT_ID=eid,
                        TYPE=0, VALUE_BRANCH_ID=0)
        self.font.add(ATTRIBUTE_ID=self.attr["F_FONT"], ELEMENT_ID=eid,
                      NAME="Dialog", SIZE=10, STYLE=0, VALUE_BRANCH_ID=0)
        self.color.add(ATTRIBUTE_ID=self.attr["F_BACKGROUND"], ELEMENT_ID=eid,
                       COLOR=-1, VALUE_BRANCH_ID=0)           # белый
        self.color.add(ATTRIBUTE_ID=self.attr["F_FOREGROUND"], ELEMENT_ID=eid,
                       COLOR=-16777216, VALUE_BRANCH_ID=0)    # чёрный
        self.decomp.add(ATTRIBUTE_ID=self.attr["F_DECOMPOSITION_TYPE"], ELEMENT_ID=eid,
                        TYPE=-1, VALUE_BRANCH_ID=0)
        return eid

    def delete_element(self, element_id) -> None:
        """Пометить элемент удалённым (removed_branch_id = 0), как это делает Ramus."""
        for r in self.elements.select(ELEMENT_ID=str(element_id)):
            r["REMOVED_BRANCH_ID"] = DEAD


# --------------------------------------------------------------------------- #
#  CLI
# --------------------------------------------------------------------------- #

def dump(path: str) -> None:
    r = Rsf(path)
    m = Idef0(r)
    print(f"файл: {path}")
    print(f"модель: элемент {m.model_element}, квалификатор работ "
          f"{m.func_qualifier} ({[q['QUALIFIER_NAME'] for q in m.qualifiers.rows if q['QUALIFIER_ID']==m.func_qualifier][0]})")
    print("\nРАБОТЫ")
    for f in m.functions():
        print(f"  {f['id']:>4}  parent={f['parent']:>3}  type={f['type']:<4} "
              f"{str(f['bounds']):<34} {f['name']}")
    print("\nПОТОКИ (стрелки-сущности)")
    for s in m.streams():
        print(f"  {s['id']:>4}  {s['name']}")
    names = {f["id"]: f["name"] for f in m.functions()}
    snames = {s["id"]: s["name"] for s in m.streams()}
    sides = {"0": "выход(R)", "1": "механизм(B)", "2": "вход(L)", "3": "управление(T)"}
    print("\nСЕКТОРЫ (сегменты стрелок)")
    for s in m.sectors():
        def side(b):
            if b is None:
                return "?"
            if b["FUNCTION"] not in (None, "-1"):
                return f"{names.get(b['FUNCTION'], b['FUNCTION'])}:{sides.get(b['FUNCTION_TYPE'], b['FUNCTION_TYPE'])}"
            if b["BORDER_TYPE"] not in (None, "-1"):
                return f"край диаграммы({b['BORDER_TYPE']})"
            return f"узел#{b['CROSSPOINT']}"
        print(f"  {s['id']:>4}  диаграмма={names.get(s['diagram_function'], s['diagram_function'])!r:<28}"
              f" поток={snames.get(s['stream'])!r:<32} {side(s['start'])} -> {side(s['end'])}")


if __name__ == "__main__":
    import sys
    if len(sys.argv) < 2:
        print(__doc__)
        print("использование: rsf.py <файл.rsf>            — распечатать модель")
        print("               rsf.py <вх.rsf> <вых.rsf>    — демо-правка")
        sys.exit(0)
    if len(sys.argv) == 2:
        dump(sys.argv[1])
    else:
        r = Rsf(sys.argv[1])
        m = Idef0(r)
        funcs = m.functions()
        root = next(f for f in funcs if f["type"] == TYPE_OPERATION or f["parent"] == m.model_element)
        m.rename_function(funcs[-1]["id"], funcs[-1]["name"] + " (изменено скриптом)")
        m.add_function(root["id"], "Новая работа из Python", x=100, y=400, w=150, h=80)
        r.save(sys.argv[2])
        print("записано:", sys.argv[2])
