def analyze_logs(logs):
    text = logs.lower()

    rules = [
        {
            "keywords": ["database connection failed", "connection refused"],
            "root_cause": "Database connectivity issue",
            "suggested_fix": "Verify database availability, hostname, port and credentials.",
            "severity": "high"
        },
        {
            "keywords": ["out of memory", "oom", "memory limit"],
            "root_cause": "Memory exhaustion",
            "suggested_fix": "Review memory usage and container memory limits.",
            "severity": "critical"
        },
        {
            "keywords": ["permission denied", "operation not permitted"],
            "root_cause": "Permission or access configuration issue",
            "suggested_fix": "Review file permissions and application user permissions.",
            "severity": "high"
        },
        {
            "keywords": ["address already in use", "port already allocated"],
            "root_cause": "Port conflict",
            "suggested_fix": "Check port mappings and identify conflicting services.",
            "severity": "medium"
        },
        {
            "keywords": ["no such file or directory", "file not found"],
            "root_cause": "Missing file or configuration",
            "suggested_fix": "Verify the required file path and configuration.",
            "severity": "medium"
        }
    ]

    for rule in rules:
        if any(keyword in text for keyword in rule["keywords"]):
            return {
                "root_cause": rule["root_cause"],
                "suggested_fix": rule["suggested_fix"],
                "severity": rule["severity"],
                "analysis_type": "rule-based"
            }

    return {
        "root_cause": "Unknown application or container failure",
        "suggested_fix": "Review the complete container logs and application configuration.",
        "severity": "high",
        "analysis_type": "rule-based-fallback"
    }
