#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""检查 WXML 里「wx:for 的循环变量名与模板引用名不一致」的静默 bug。

背景（真实踩坑）：
    WXML 的 wx:for 默认把当前项命名为 `item`（下标 `index`）。若模板里写 `badge.xxx`
    而该元素没有声明 `wx:for-item="badge"`，所有 `badge.xxx` 会静默求值为 undefined：

    - `{{badge.name}}`            → 渲染为空字符串
    - `wx:if="{{badge.iconURL}}"` → false，图标整块不渲染
    - `wx:if="{{!badge.locked}}"` → `!undefined === true`，**反向命中**，未点亮也显示「已点亮」

    页面不报错、控制台无异常，只是"什么都看不见 + 状态全反"，极难排查。

用法：
    python3 scripts/check-wxml-loopvars.py [miniprogram 目录]     # 默认 ./miniprogram
退出码：0 = 无问题；1 = 发现可疑位置（可直接用于 CI / make verify）
"""
import os
import re
import sys
import glob

TAG_RE = re.compile(r'<(/?)([a-zA-Z][a-zA-Z0-9-]*)((?:"[^"]*"|\'[^\']*\'|[^>"\'])*?)(/?)>')
ATTR_RE = re.compile(r'([a-zA-Z_:][\w:.-]*)\s*=\s*"([^"]*)"')
EXPR_RE = re.compile(r'\{\{(.*?)\}\}', re.S)
QUOTED_RE = re.compile(r'"[^"]*"|\'[^\']*\'')
# 只取链式访问的「根」标识符：排除 a.b 里的 b，也排除 xxx.svg 里的 svg
ROOT_RE = re.compile(r'(?<![\w$.])([A-Za-z_$][\w$]*)\s*\.')
VOID = ('image', 'input', 'icon', 'progress', 'slider', 'switch', 'textarea')


def page_data_keys(js_path):
    """宽松收集页面 JS 里出现过的 `foo:` 键名——宁可漏报，不可误报。"""
    if not os.path.exists(js_path):
        return set()
    with open(js_path, encoding='utf-8') as fh:
        return set(re.findall(r'\b([A-Za-z_$][\w$]*)\s*:', fh.read()))


def build_spans(src):
    """每个元素的 (start, end)，用来取 wx:for 元素的子树范围。"""
    stack, spans = [], []
    for m in TAG_RE.finditer(src):
        closing, name, selfclose = m.group(1), m.group(2), m.group(4)
        if closing:
            if stack and stack[-1][0] == name:
                _, start = stack.pop()
                spans.append((start, m.end()))
        elif selfclose or name in VOID:
            spans.append((m.start(), m.end()))
        else:
            stack.append((name, m.start()))
    return spans


def scan(path):
    with open(path, encoding='utf-8') as fh:
        src = fh.read()
    body = re.sub(r'<!--.*?-->', '', src, flags=re.S)
    spans = build_spans(body)
    tokens = [(m.start(), m.group(1), dict(ATTR_RE.findall(m.group(3))))
              for m in TAG_RE.finditer(body)]

    declared = set()
    for _, _, attrs in tokens:
        for key in ('wx:for-item', 'wx:for-index'):
            if attrs.get(key):
                declared.add(attrs[key])

    data_keys = page_data_keys(path[:-5] + '.js') | {'true', 'false', 'null', 'undefined', 'wx'}

    findings = []
    for start, closing, attrs in tokens:
        if closing or not attrs.get('wx:for'):
            continue
        item = attrs.get('wx:for-item') or 'item'
        index = attrs.get('wx:for-index') or 'index'
        end = next((e for s, e in spans if s == start), start + 4000)
        block = body[start:end]

        roots = set()
        for em in EXPR_RE.finditer(block):
            expr = QUOTED_RE.sub('""', em.group(1))  # 去掉字符串字面量（避免把 *.svg 当变量）
            for rm in ROOT_RE.finditer(expr):
                root = rm.group(1)
                if root not in (item, index):
                    roots.add(root)

        bad = sorted(r for r in roots
                     if r not in data_keys and r not in declared and r not in ('item', 'index'))
        if bad:
            findings.append((body[:start].count('\n') + 1, attrs['wx:for'], item, bad))
    return findings


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else 'miniprogram'
    total = 0
    for p in sorted(glob.glob(os.path.join(root, '**/*.wxml'), recursive=True)):
        if 'node_modules' in p or 'miniprogram_npm' in p:
            continue
        res = scan(p)
        if res:
            total += len(res)
            print('⚠', p)
            for line, expr, item, bad in res:
                print(f'   line {line}: wx:for="{expr}" → 本层循环变量为 "{item}"，模板却引用了 {bad}')
    if total:
        print(f'\n发现 {total} 处可疑：请补 wx:for-item="<你用的名字>"，或把模板引用改成 item/index')
        return 1
    print('WXML 循环变量检查通过')
    return 0


if __name__ == '__main__':
    sys.exit(main())
