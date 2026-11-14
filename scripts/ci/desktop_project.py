"""Launch a copied starter bundle; require a visible window and embedded Web assets.

This is a bounded native startup probe, not keyboard/VoiceOver acceptance.
"""
import html
import json
import pathlib
import re
import subprocess
import sys
import tempfile
import time
import urllib.request

project = pathlib.Path(sys.argv[1]).resolve()
window_probe = pathlib.Path(sys.argv[2]).resolve()
kind = sys.argv[3]
if kind not in ("go", "webview"):
    raise SystemExit("expected go or webview")
config = json.loads((project / "omnigo.json").read_text())
bundle = config["bundle"]
source = project / bundle["destination"] / (bundle["name"] + ".app")
with tempfile.TemporaryDirectory(prefix="omnigo-installed-") as installation:
    app = pathlib.Path(installation) / source.name
    subprocess.run(["ditto", str(source), str(app)], check=True)
    subprocess.run(["codesign", "--verify", "--strict", str(app)], check=True)
    with (project / "native-startup.log").open("w") as log:
        child = subprocess.Popen([str(app / "Contents/MacOS/app")], cwd="/", stdout=log, stderr=log)
        try:
            deadline = time.monotonic() + 20
            while True:
                if child.poll() is not None:
                    raise RuntimeError(f"starter exited before showing a window: {child.returncode}")
                result = subprocess.run([str(window_probe), str(child.pid)], capture_output=True, text=True, timeout=5)
                if result.returncode == 0:
                    print(result.stdout.strip())
                    break
                if time.monotonic() >= deadline:
                    raise RuntimeError("no visible native window before deadline")
                time.sleep(0.1)
            if kind == "webview":
                ports = subprocess.check_output(["lsof", "-Pan", "-p", str(child.pid), "-iTCP", "-sTCP:LISTEN", "-Fn"], text=True, timeout=5)
                port = re.search(r"n127\.0\.0\.1:(\d+)", ports).group(1)
                with urllib.request.urlopen("http://127.0.0.1:" + port + "/", timeout=5) as response:
                    body = response.read().decode()
                    if "Scratchpad" not in body or bundle["name"] not in html.unescape(body):
                        raise RuntimeError("missing embedded starter page")
                print("PASS: embedded WebView assets served outside checkout")
        finally:
            child.terminate()
            try:
                child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()
