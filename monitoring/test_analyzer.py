from analyzers.root_cause import analyze_logs


test_cases = [
    "Database connection failed",
    "Out of memory: memory limit exceeded",
    "Permission denied while accessing config",
    "Address already in use",
    "No such file or directory",
    "Some unknown application error"
]

for log in test_cases:
    result = analyze_logs(log)

    print("\nLog:", log)
    print("Root Cause:", result["root_cause"])
    print("Suggested Fix:", result["suggested_fix"])
    print("Severity:", result["severity"])
    print("Analysis:", result["analysis_type"])
