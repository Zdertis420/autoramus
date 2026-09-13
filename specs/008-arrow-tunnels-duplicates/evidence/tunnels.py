#!/usr/bin/env python3
"""Считает по правилам Ramus, какие концы секторов будут нарисованы туннелем.

Правило собрано из исходников Ramus:
  AbstractCrosspoint.getTunnelType + PaintSector.get{Start,End}TunnelType.
"""
import sys, zipfile, io
import xml.etree.ElementTree as ET
from collections import defaultdict


def table(z, name):
    for n in z.namelist():
        if n.endswith('/' + name + '.xml'):
            root = ET.parse(io.BytesIO(z.read(n))).getroot()
            names = {f.get('id'): f.get('name') for f in root.find('fields')}
            return [{names[f.get('id')]: (f.text or '') for f in r} for r in root.find('data')]
    return []


def load(path):
    z = zipfile.ZipFile(path)
    m = {}
    m['attributes'] = table(z, 'attributes')
    m['elements'] = table(z, 'elements')
    m['qualifiers'] = table(z, 'qualifiers')
    m['others'] = table(z, 'attribute_other_elements')
    m['hier'] = table(z, 'attribute_hierarchicals')
    m['borders'] = table(z, 'attribute_sector_borders')
    m['texts'] = table(z, 'attribute_texts')
    return m


def analyse(path):
    m = load(path)
    attr = {r['ATTRIBUTE_NAME']: r['ATTRIBUTE_ID'] for r in m['attributes'] if r.get('ATTRIBUTE_NAME')}
    qual = {r['QUALIFIER_NAME']: r['QUALIFIER_ID'] for r in m['qualifiers'] if r.get('QUALIFIER_NAME')}

    alive = {}
    for r in m['elements']:
        if r.get('REMOVED_BRANCH_ID', '2147483647') == '2147483647':
            alive[r['ELEMENT_ID']] = r['QUALIFIER_ID']

    sectors_q = qual.get('F_SECTORS')
    base_q = qual.get('F_BASE_FUNCTIONS')

    # диаграмма сектора и его поток
    fs, ss = attr.get('F_FUNCTION_SECTOR'), attr.get('F_SECTOR_STREAM')
    diagram, stream = {}, {}
    for r in m['others']:
        if r['ATTRIBUTE_ID'] == fs:
            diagram[r['ELEMENT_ID']] = r['OTHER_ELEMENT']
        elif r['ATTRIBUTE_ID'] == ss:
            stream[r['ELEMENT_ID']] = r['OTHER_ELEMENT']

    # иерархия работ
    parent, children = {}, defaultdict(int)
    hier_attr = attr.get('F_FUNCTIONS_HIERARCHY') or attr.get('HierarchicalAttribute')
    for r in m['hier']:
        eid = r['ELEMENT_ID']
        p = r.get('PARENT_ELEMENT_ID', '-1')
        if eid in alive:
            parent[eid] = p
            if p != '-1' and p != '':
                children[p] += 1

    # имена (работ и потоков)
    name = {}
    for r in m['texts']:
        name.setdefault(r['ELEMENT_ID'], r.get('VALUE', ''))

    # концы секторов
    start_a, end_a = attr.get('F_SECTOR_BORDER_START'), attr.get('F_SECTOR_BORDER_END')
    ends = {}   # (sector, 'start'|'end') -> row
    ins, outs = defaultdict(list), defaultdict(list)
    for r in m['borders']:
        side = 'start' if r['ATTRIBUTE_ID'] == start_a else 'end'
        sec = r['ELEMENT_ID']
        if sec not in alive:
            continue
        ends[(sec, side)] = r
        cp = r.get('CROSSPOINT', '-1')
        if cp and cp not in ('-1', ''):
            (outs if side == 'start' else ins)[cp].append(sec)

    print(f'=== {path}')
    tunnels = []
    for (sec, side), r in sorted(ends.items(), key=lambda kv: (int(kv[0][0]), kv[0][1])):
        cp = r.get('CROSSPOINT', '-1')
        i, o = len(ins.get(cp, [])), len(outs.get(cp, []))
        # AbstractCrosspoint.getTunnelType: скобки, если у узла нет входов или нет выходов
        res = 'NONE' if (i > 0 and o > 0) else 'HARD'
        # PaintSector: диаграмма контекста (родитель — F_BASE_FUNCTIONS) + конец на краю -> NONE
        d = diagram.get(sec)
        # PaintSector: «родитель диаграммы называется F_BASE_FUNCTIONS» —
        # то есть диаграмма контекстная (A-0): её работа лежит в квалификаторе
        # F_BASE_FUNCTIONS, а не в квалификаторе работ модели.
        context = alive.get(d) == base_q
        if context and int(r.get('BORDER_TYPE', '-1') or -1) >= 0:
            res = 'NONE'
        # PaintSector: конец на блоке без декомпозиции -> NONE
        fn = r.get('FUNCTION', '-1')
        if res != 'NONE' and fn not in ('-1', '') and children.get(fn, 0) == 0:
            res = 'NONE'
        if res != 'NONE':
            tunnels.append((sec, side, cp, i, o, r.get('BORDER_TYPE'), fn,
                            name.get(stream.get(sec, ''), '?'),
                            name.get(d, d)))
    if not tunnels:
        print('  туннелей нет')
    for t in tunnels:
        print('  сектор %-4s %-5s cp=%-4s ins=%d outs=%d border=%-3s func=%-4s поток=%-28s диаграмма=%s' % t)
    return tunnels


for p in sys.argv[1:]:
    analyse(p)
