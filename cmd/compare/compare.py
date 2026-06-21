#!/usr/bin/env python3
"""
对比报告生成器：比较 Go 与 Python 缠论实现的 JSON 输出。
"""
import json
import sys
import os

def load_json(path):
    with open(path) as f:
        return json.load(f)

def compare_merged_klines(go, py):
    print(f"\n{'='*60}")
    print(f"1. 合并K线")
    print(f"{'='*60}")
    print(f"  Go: {len(go)} 条, Python: {len(py)} 条")
    if len(go) != len(py):
        print(f"  ⚠️  数量不同!")
    mismatches = 0
    for i in range(min(len(go), len(py))):
        g, p = go[i], py[i]
        if abs(g['high'] - p['high']) > 0.01 or abs(g['low'] - p['low']) > 0.01:
            if mismatches < 5:
                print(f"  [{i}] Go: H={g['high']:.2f} L={g['low']:.2f}  Py: H={p['high']:.2f} L={p['low']:.2f}")
            mismatches += 1
    if mismatches > 0:
        print(f"  ⚠️  {mismatches} 条K线价格不同!")
    else:
        print(f"  ✅ 合并K线一致")

def compare_bis(go, py):
    print(f"\n{'='*60}")
    print(f"2. 笔 (Bi)")
    print(f"{'='*60}")
    print(f"  Go: {len(go)} 笔, Python: {len(py)} 笔")
    if len(go) != len(py):
        print(f"  ⚠️  数量不同! 差异={len(py)-len(go)}")
    
    # Compare by index
    for i in range(min(len(go), len(py))):
        g, p = go[i], py[i]
        diffs = []
        if g.get('direction') != p.get('direction'):
            diffs.append(f"dir: {g.get('direction')} vs {p.get('direction')}")
        if abs(g.get('start_price', 0) - p.get('start_price', 0)) > 1:
            diffs.append(f"start_price: {g.get('start_price'):.2f} vs {p.get('start_price'):.2f}")
        if abs(g.get('end_price', 0) - p.get('end_price', 0)) > 1:
            diffs.append(f"end_price: {g.get('end_price'):.2f} vs {p.get('end_price'):.2f}")
        if g.get('start_index') != p.get('start_index'):
            diffs.append(f"start_index: {g.get('start_index')} vs {p.get('start_index')}")
        if g.get('end_index') != p.get('end_index'):
            diffs.append(f"end_index: {g.get('end_index')} vs {p.get('end_index')}")
        if diffs:
            print(f"  bi[{i}]: {'; '.join(diffs)}")

def compare_segments(go, py):
    print(f"\n{'='*60}")
    print(f"3. 线段 (Segment)")
    print(f"{'='*60}")
    print(f"  Go: {len(go)} 段, Python: {len(py)} 段")
    if len(go) != len(py):
        print(f"  ⚠️  数量不同! 差异={len(py)-len(go)}")
    
    for i in range(min(len(go), len(py))):
        g, p = go[i], py[i]
        g_dir = "up" if g.get('direction') in (1, "1", "up") else "down"
        p_dir = p.get('direction', '')
        g_sure = g.get('is_sure', g.get('IsSure', False))
        p_sure = p.get('is_sure', False)
        
        diffs = []
        if g_dir != p_dir:
            diffs.append(f"dir: {g_dir} vs {p_dir}")
        if g_sure != p_sure:
            diffs.append(f"sure: {g_sure} vs {p_sure}")
        if g.get('StartIndex', g.get('start_index')) != p.get('start_index'):
            diffs.append(f"start: {g.get('StartIndex', g.get('start_index'))} vs {p.get('start_index')}")
        if g.get('EndIndex', g.get('end_index')) != p.get('end_index'):
            diffs.append(f"end: {g.get('EndIndex', g.get('end_index'))} vs {p.get('end_index')}")
        if diffs:
            print(f"  seg[{i}]: {'; '.join(diffs)}")
        else:
            g_start = g.get('start_bi_idx', g.get('StartIndex', '?'))
            g_end = g.get('end_bi_idx', g.get('EndIndex', '?'))
            p_start = p.get('start_bi_idx', '?')
            p_end = p.get('end_bi_idx', '?')
            print(f"  seg[{i}]: ✅ dir={g_dir} sure={g_sure} bi[{g_start}:{g_end}] vs bi[{p_start}:{p_end}]")

def compare_pivots(go, py):
    print(f"\n{'='*60}")
    print(f"4. 中枢 (Pivot)")
    print(f"{'='*60}")
    print(f"  Go: {len(go)} 个, Python: {len(py)} 个")
    if len(go) != len(py):
        print(f"  ⚠️  数量不同!")
    
    for i in range(min(len(go), len(py))):
        g, p = go[i], py[i]
        diffs = []
        for field in ['ZG', 'ZD', 'GG', 'DD']:
            g_field = field if field in g else (field.lower() if field.lower() in g else 'peak_high' if field == 'GG' else 'peak_low' if field == 'DD' else None)
            if g_field is None:
                continue
            gv = g.get(g_field, g.get(field.lower(), 0))
            pv = p.get(field, p.get(field.lower(), 0))
            if abs(gv - pv) > 0.1:
                diffs.append(f"{field}: {gv:.2f} vs {pv:.2f}")
        if diffs:
            print(f"  zs[{i}]: {'; '.join(diffs)}")
        else:
            print(f"  zs[{i}]: ✅")

def compare_signals(go, py):
    print(f"\n{'='*60}")
    print(f"5. 信号 (Signal)")
    print(f"{'='*60}")
    print(f"  Go: {len(go)} 个, Python: {len(py)} 个")
    if len(go) != len(py):
        print(f"  ⚠️  数量不同!")
    
    print("  Go signals:")
    for s in go:
        print(f"    type={s.get('Type', s.get('type'))} sub={s.get('SubType', s.get('sub'))} idx={s.get('Index', s.get('index'))} price={s.get('Price', s.get('price')):.2f}")
    print("  Python signals:")
    for s in py:
        print(f"    type={s.get('type')} is_buy={s.get('is_buy')} bi={s.get('bi_idx')} price={s.get('price'):.2f}")

def main():
    go_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "../../../go_result.json")
    py_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "../../../../chan.py/py_result.json")
    
    go = load_json(go_path)
    py = load_json(py_path)
    
    print("缠论实现对比报告：Go vs Python")
    print("=" * 60)
    print(f"输入: 200 条合成K线 (正弦波)")
    
    compare_merged_klines(
        go.get('merged_klines', go.get('MergedKlines', [])),
        py.get('merged_klines', [])
    )
    compare_bis(
        go.get('bis', go.get('Bis', [])),
        py.get('bis', [])
    )
    compare_segments(
        go.get('segments', go.get('Segments', [])),
        py.get('segments', [])
    )
    compare_pivots(
        go.get('pivots', go.get('Pivots', [])),
        py.get('pivots', [])
    )
    compare_signals(
        go.get('signals', go.get('Signals', [])),
        py.get('signals', [])
    )

if __name__ == '__main__':
    main()
