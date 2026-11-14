"""Sample an existing macOS process; no input injection or process termination.
Usage: python3 scripts/measure/idle.py PID [seconds]
RSS is KiB; ps %CPU is an OS averaged indicator, not per-interval CPU time.
"""
import csv
import subprocess
import sys
import time
pid = int(sys.argv[1])
seconds = int(sys.argv[2]) if len(sys.argv) > 2 else 60
writer = csv.writer(sys.stdout)
writer.writerow(["elapsed_s", "cpu_percent", "rss_kib"])
start = time.monotonic()
for _ in range(seconds):
    values = subprocess.check_output(
        ["ps", "-p", str(pid), "-o", "%cpu=", "-o", "rss="], text=True
    ).split()
    if len(values) != 2 or int(values[1]) <= 0:
        raise SystemExit("Process no longer available")
    writer.writerow([round(time.monotonic()-start, 3), *values])
    sys.stdout.flush()
    time.sleep(1)
