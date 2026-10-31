import json
import subprocess
import sys
import urllib.request
import urllib.error


API_URL = "http://localhost:8081/incidents"


def collect_logs(container_name):
    try:
        result = subprocess.run(
            ["docker", "logs", "--tail", "50", container_name],
            capture_output=True,
            text=True,
            timeout=5
        )

        logs = result.stdout + result.stderr

        if not logs.strip():
            return "No container logs available."

        return logs[-10000:]

    except subprocess.TimeoutExpired:
        return "Log collection timed out."

    except Exception as error:
        return f"Log collection failed: {error}"


def create_incident(name, severity, exit_code, event_type, logs):
    payload = {
        "title": f"OpsMind Alert: {event_type} - {name}",
        "description": f"Docker detected {event_type} for container {name}",
        "container_name": name,
        "severity": severity,
        "status": "open",
        "root_cause": (
            f"Container event: {event_type}, Exit Code: {exit_code}"
        ),
        "suggested_fix": "Review application logs and container configuration",
        "logs": logs
    }

    data = json.dumps(payload).encode("utf-8")

    request = urllib.request.Request(
        API_URL,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            result = json.loads(response.read().decode("utf-8"))
            print("[INCIDENT CREATED]")
            print("Incident ID:", result.get("id"))
            print("Title:", result.get("title"))

    except urllib.error.HTTPError as error:
        print("[API ERROR]", error.code, error.read().decode("utf-8"))

    except urllib.error.URLError as error:
        print("[CONNECTION ERROR]", error)


def start_monitor():
    print("================================")
    print("     OpsMind Docker Monitor")
    print("================================")
    print("Listening for Docker events...")
    print("Monitoring only opsmind- containers.")
    print("Press CTRL+C to stop.\n")

    stopped_containers = set()

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

    try:
        process = subprocess.Popen(
            command,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            bufsize=1
        )

        for line in process.stdout:
            if not line.strip():
                continue

            event = json.loads(line)

            action = event.get("Action")
            attributes = event.get("Actor", {}).get("Attributes", {})
            name = attributes.get("name", "unknown")

            if not name.startswith("opsmind-"):
                continue

            if action == "start":
                stopped_containers.discard(name)
                print("[STARTED]", name)

            elif action == "stop":
                stopped_containers.add(name)
                print("[STOPPED]", name)

            elif action == "die":
                if name in stopped_containers:
                    stopped_containers.discard(name)
                    print("[DUPLICATE IGNORED]", name)
                    continue

                exit_code = attributes.get("exitCode", "unknown")

                if exit_code == "0":
                    print("[NORMAL EXIT]", name)

                elif exit_code == "unknown":
                    print("[UNCLASSIFIED EXIT]", name)

                else:
                    print("[POTENTIAL CRASH]", name)
                    print("Exit Code:", exit_code)

                    logs = collect_logs(name)
                    print("[LOGS COLLECTED]")
                    print(logs)

                    create_incident(
                        name,
                        "high",
                        exit_code,
                        "Container Crash",
                        logs
                    )

            elif action == "oom":
                print("[CRITICAL EVENT] Out of Memory:", name)

                logs = collect_logs(name)

                create_incident(
                    name,
                    "critical",
                    "unknown",
                    "OOM",
                    logs
                )

    except FileNotFoundError:
        print("Error: Docker CLI not found.")
        sys.exit(1)

    except KeyboardInterrupt:
        print("\nMonitoring stopped.")

    except Exception as error:
        print("Monitoring error:", error)


if __name__ == "__main__":
    start_monitor()
