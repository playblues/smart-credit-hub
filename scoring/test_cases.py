"""Запуск: python -m pytest scoring/ -q  (или python scoring/test_cases.py)"""
import json, pathlib
from engine import evaluate

CASES = json.loads((pathlib.Path(__file__).parent / "testdata" / "cases.json").read_text(encoding="utf-8"))

def test_all_cases():
    for c in CASES:
        got = evaluate({k: c[k] for k in ("company", "bki", "request")})
        assert got == c["expected_result"], f'{c["case_id"]}: {got} != {c["expected_result"]}'

if __name__ == "__main__":
    test_all_cases(); print(f"OK: {len(CASES)} кейсов сошлись")
