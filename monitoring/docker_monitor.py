import hashlib
import json
import os
import subprocess
import urllib.request
import urllib.error

from analyzers.root_cause import analyze_logs


API_URL = "http://localhost:8081/monitoring/events"
MONITORING_TOKEN = os.environ.get("OPSMIND_MONITORING_TOKEN", "")
CONTAINER_PREFIX = "opsmind-"


def collect_logs(container_name):
    """Collect the last 50 lines of container logs."""

    try:
        result = subprocess.run(
            ["docker", "logs", "--tail", "50", container_name],
            capture_output=True,
            text=True,
            timeout=5
        )

        logs = (result.stdout or "") + (result.stderr or "")
        return logs[-10000:]

    except subprocess.TimeoutExpired:
        return "Log collection timed out."

    except Exception as error:
        return f"Log collection failed: {error}"


def create_incident(name, severity, exit_code, event_type, logs, analysis):
    """Send an authenticated monitoring event to the DevBoard API."""

    fingerprint_input = f"{name}|{event_type}"
    fingerprint = hashlib.sha256(
        fingerprint_input.encode("utf-8")
    ).hexdigest()

    payload = {
        "title": f"OpsMind Alert: {event_type} - {name}",
        "description": (
            f"Container {name} generated a {event_type} event. "
            f"Exit code: {exit_code}"
        ),
        "container_name": name,
        "severity": severity,
        "root_cause": analysis["root_cause"],
        "suggested_fix": analysis["suggested_fix"],
        "logs": logs,
        "fingerprint": fingerprint
    }

    data = json.dumps(payload).encode("utf-8")

    request = urllib.request.Request(
        API_URL,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {MONITORING_TOKEN}"
        },
        method="POST"
    )

    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            result = response.read().decode("utf-8")
            print("[MONITORING API RESPONSE]")
            print(result)

    except urllib.error.HTTPError as error:
        print("[API ERROR]", error.code, error.read().decode("utf-8"))

    except urllib.error.URLError as error:
        print("[CONNECTION ERROR]", error)


def start_monitor():
    """Listen for Docker events and analyze OpsMind test containers."""

    if not MONITORING_TOKEN:
        print("[ERROR] OPSMIND_MONITORING_TOKEN environment variable is missing.")
        return

    print("===================================")
    print("       OpsMind Docker Monitor")
    print("===================================")
    print("Monitoring containers with prefix:", CONTAINER_PREFIX)
    print("API endpoint:", API_URL)
    print("Press Ctrl+C to stop monitoring.")

    command = [
        "docker",
        "events",
        "--filter", "type=container",
        "--filter", "event=die",
        "--filter", "event=oom",
        "--filter", "event=stop",
        "--filter", "event=start",
        "--format", "{{json .}}"
    ]

    stopped_containers = set()

    try:
        process = subprocess.Popen(
            command,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            bufsize=1
        )

        if process.stdout is None:
            print("Unable to read Docker events.")
            return

        for line in process.stdout:
            if not line.strip():
                continue

            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue

            action = event.get("Action", event.get("status", ""))
            attributes = event.get("Actor", {}).get("Attributes", {})
            name = attributes.get("name", "")
            exit_code = attributes.get("exitCode", "unknown")

            if not name.startswith(CONTAINER_PREFIX):
                continue

            print(f"\n[DOCKER EVENT] {action} - {name}")

            if action == "start":
                stopped_containers.discard(name)
                continue

            if action == "stop":
                stopped_containers.add(name)
                continue

            if action == "die":
                if name in stopped_containers:
                    stopped_containers.discard(name)
                    continue

                if exit_code == "0":
                    print("[INFO] Container exited normally.")
                    continue

                logs = collect_logs(name)
                analysis = analyze_logs(logs, use_llm=True)

                print("\n[ROOT CAUSE ANALYSIS]")
                print("Root Cause:", analysis["root_cause"])
                print("Suggested Fix:", analysis["suggested_fix"])
                print("Severity:", analysis["severity"])

                create_incident(
                    name,
                    analysis["severity"],
                    exit_code,
                    "Container Crash",
                    logs,
                    analysis
                )

            elif action == "oom":
                logs = collect_logs(name)
                analysis = analyze_logs(logs, use_llm=True)
                analysis["severity"] = "critical"

                print("\n[OOM DETECTED]")
                print("Root Cause:", analysis["root_cause"])
                print("Suggested Fix:", analysis["suggested_fix"])

                create_incident(
                    name,
                    "critical",
                    exit_code,
                    "OOM",
                    logs,
                    analysis
                )

    except KeyboardInterrupt:
        print("\n[INFO] Monitoring stopped.")

    except FileNotFoundError:
        print("[ERROR] Docker command not found.")

    except Exception as error:
        print("[ERROR]", error)


if __name__ == "__main__":
    start_monitor()
