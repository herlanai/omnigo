"""Require named native tests to pass; skipped/missing tests are failures."""
import json
import sys


def verify(lines, required):
    passed = set()
    failed = set()
    for line in lines:
        event = json.loads(line)
        name = event.get('Test')
        if event.get('Action') == 'pass' and name:
            passed.add(name)
        if event.get('Action') in ('fail', 'skip') and name in required:
            failed.add(name)
        if event.get('Action') == 'fail' and not name:
            failed.add(event.get('Package', 'package'))
    missing = set(required) - passed
    if missing or failed:
        raise ValueError(f'native verification failed: missing={sorted(missing)}, failed={sorted(failed)}')


if __name__ == '__main__':
    if len(sys.argv) < 3:
        raise SystemExit('usage: require_native_tests.py LOG TEST...')
    with open(sys.argv[1], encoding='utf-8') as source:
        verify(source, sys.argv[2:])
