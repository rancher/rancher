#!/usr/bin/env python3
import os
import sys
import requests
import argparse
import re
import urllib.parse
from concurrent.futures import ThreadPoolExecutor, as_completed

def make_github_request(url, max_retries=3):
    GH_TOKEN = os.environ.get("GH_TOKEN")
    if not GH_TOKEN:
        print("Error: GH_TOKEN environment variable not set")
        sys.exit(1)

    headers = {
        "Authorization": f"Bearer {GH_TOKEN}",
        "Accept": "application/vnd.github.v3+json"
    }

    retry_count = 0
    while retry_count < max_retries:
        try:
            response = requests.get(url, headers=headers, timeout=30)
            return response
        except (requests.RequestException, requests.Timeout):
            retry_count += 1
            if retry_count >= max_retries:
                raise
    raise Exception(f"Failed to get response from {url} after {max_retries} retries")

def get_job_logs(repo_owner, repo_name, job_id):
    logs_url = f"https://api.github.com/repos/{repo_owner}/{repo_name}/actions/jobs/{job_id}/logs"
    logs_response = make_github_request(logs_url)

    if logs_response.status_code != 200:
        # Don't print error for common cases like 404 (logs not available)
        # This can happen for re-run jobs or expired logs
        return None

    return logs_response.text

def extract_failure_lines(log_content, max_lines=10):
    env_include = os.environ.get("INCLUDE_PATTERNS")
    default_include = [
        'FAIL', 'Fail', 'failed',
        'ERROR:', 'Error:', 'error:', '##[error]',
        'Error Trace:',
    ]

    include_patterns = [urllib.parse.unquote(pattern.strip()) for pattern in env_include.split(',')] if env_include else default_include

    env_exclude = os.environ.get("EXCLUDE_PATTERNS")
    default_exclude = [
        'Failed to save: Unable to reserve cache with key docker.io',
        'Failed to restore: "/usr/bin/tar" failed with error: The process',
        'echo "Error: Failed to load image from tarball!"',
        'H4sIAAAAAAAA',
    ]

    exclude_patterns = [urllib.parse.unquote(pattern.strip()) for pattern in env_exclude.split(',')] if env_exclude else default_exclude

    failure_lines = []
    seen_normalized_lines = set()

    def normalize_line(line):
        """Remove timestamps and other variable parts from a line."""
        # Remove ISO 8601 timestamps with milliseconds
        line = re.sub(r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z', '', line)

        # Remove step indicators (#N) and time values
        line = re.sub(r'#\d+\s+\d+\.\d+\s+', '', line)

        # Handle complex error codes with process IDs and timestamps in brackets
        line = re.sub(r'\[\d+:\d+/\d+\.\d+:ERROR:[^]]+\]', '', line)

        # Handle WebGL identifiers with hexadecimal numbers
        line = re.sub(r'\[\.WebGL-0x[0-9a-f]+\]', '', line)

        # Handle other timestamp patterns
        line = re.sub(r'\[\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}([.,]\d+)?Z?\]', '', line)
        line = re.sub(r'\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}([.,]\d+)?Z?', '', line)
        line = re.sub(r'\d{2}:\d{2}:\d{2}([.,]\d+)?', '', line)
        line = re.sub(r'(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}', '', line)

        # Remove floating point numbers at the beginning (often execution times)
        line = re.sub(r'^\s*\d+\.\d+\s+', '', line)

        # Remove line numbers that often change between similar errors
        line = re.sub(r'line \d+', 'line', line)
        line = re.sub(r':\d+:', ':', line)

        # Remove variable paths or IDs
        line = re.sub(r'[0-9a-f]{7,40}', 'ID', line)

        return line.strip()

    def is_matching_line(line_content):
        return any(pattern in line_content for pattern in include_patterns) and not any(pattern in line_content for pattern in exclude_patterns)

    log_lines = log_content.splitlines()

    for line in reversed(log_lines):
        # Remove ANSI color code
        cleaned = re.sub(r'\x1b\[[0-9;]*m', '', line).strip()
        if not is_matching_line(cleaned):
            continue
        normalized = normalize_line(cleaned)
        # Filter duplicates
        if not normalized or normalized in seen_normalized_lines:
            continue
        seen_normalized_lines.add(normalized)
        failure_lines.append(cleaned)
        if len(failure_lines) >= max_lines:
            break

    return list(reversed(failure_lines))

FAILURE_STATES = {"failure", "timed_out", "stale"}

def process_job(repo_owner, repo_name, job, run_data, attempt_number):
    conclusion = job.get("conclusion")

    # Only process jobs with known failure conclusions
    if conclusion not in FAILURE_STATES:
        return None

    job_logs = get_job_logs(repo_owner, repo_name, job["id"])

    if job_logs is None:
        failure_lines = []
    else:
        failure_lines = extract_failure_lines(job_logs)

    log_url = f"https://github.com/{repo_owner}/{repo_name}/actions/runs/{run_data['id']}/job/{job['id']}"

    return {
        "workflow_name": run_data["name"],
        "job_name": job["name"],
        "attempt_number": attempt_number,
        "job_id": job["id"],
        "log_url": log_url,
        "failure_lines": failure_lines,
        "run_id": run_data["id"],
        "run_number": run_data["run_number"],
        "created_at": run_data["created_at"],
        "html_url": run_data["html_url"],
    }

def get_jobs_for_run(repo_owner, repo_name, run_id, attempt_number):
    base_url = f"https://api.github.com/repos/{repo_owner}/{repo_name}/actions/runs/{run_id}"
    if attempt_number > 1:
        base_url += f"/attempts/{attempt_number}"

    jobs_url = f"{base_url}/jobs?per_page=100&filter=all"
    all_jobs = []
    page = 1

    while True:
        paged_url = f"{jobs_url}&page={page}"
        jobs_response = make_github_request(paged_url)

        if jobs_response.status_code != 200:
            break

        jobs_data = jobs_response.json()
        jobs = jobs_data.get("jobs", [])

        if not jobs:
            break

        all_jobs.extend(jobs)

        if len(jobs) < 100:
            break

        page += 1

    if not all_jobs:
        direct_jobs_url = f"https://api.github.com/repos/{repo_owner}/{repo_name}/actions/jobs?run_id={run_id}&per_page=100"
        direct_response = make_github_request(direct_jobs_url)
        if direct_response.status_code == 200:
            all_jobs = direct_response.json().get("jobs", [])

    return all_jobs

def process_attempt(repo_owner, repo_name, run, attempt_number):
    if attempt_number != run.get("run_attempt", 1):
        attempt_url = f"https://api.github.com/repos/{repo_owner}/{repo_name}/actions/runs/{run['id']}/attempts/{attempt_number}"
        attempt_response = make_github_request(attempt_url)

        if attempt_response.status_code != 200:
            return []

        run_data = attempt_response.json()
    else:
        run_data = run

    jobs = get_jobs_for_run(repo_owner, repo_name, run_data["id"], attempt_number)
    if not jobs:
        return []

    attempt_failures = []
    with ThreadPoolExecutor(max_workers=min(10, len(jobs))) as executor:
        futures = {executor.submit(process_job, repo_owner, repo_name, job, run_data, attempt_number): job for job in jobs}

        for future in as_completed(futures):
            try:
                result = future.result()
                if result:
                    attempt_failures.append(result)
            except Exception as exc:
                job = futures[future]
                print(f"Error processing job {job['name']}: {exc}")

    return attempt_failures

def get_failed_workflows(repo_owner, repo_name, pr_number):
    pr_url = f"https://api.github.com/repos/{repo_owner}/{repo_name}/pulls/{pr_number}"
    pr_response = make_github_request(pr_url)

    if pr_response.status_code != 200:
        print(f"Error fetching Pull Request details: {pr_response.status_code}")
        sys.exit(1)

    pr_data = pr_response.json()
    head_sha = pr_data["head"]["sha"]
    base_branch = pr_data["base"]["ref"]

    runs_url = f"https://api.github.com/repos/{repo_owner}/{repo_name}/actions/runs?head_sha={head_sha}"
    runs_response = make_github_request(runs_url)

    if runs_response.status_code != 200:
        print(f"Error fetching workflow runs: {runs_response.status_code}")
        sys.exit(1)

    runs_data = runs_response.json()
    all_attempts = []

    for run in runs_data.get("workflow_runs", []):
        total_attempts = run.get("run_attempt", 1)
        for attempt_number in range(1, total_attempts + 1):
            all_attempts.append((run, attempt_number))

    if not all_attempts:
        return []

    failed_attempts = []
    seen_job_ids = set()

    with ThreadPoolExecutor(max_workers=min(10, len(all_attempts))) as executor:
        futures = {
            executor.submit(process_attempt, repo_owner, repo_name, run, attempt_number):
            (run["name"], attempt_number) for run, attempt_number in all_attempts
        }

        for future in as_completed(futures):
            try:
                attempt_results = future.result()
                for result in attempt_results:
                    job_id = result.get("job_id")
                    # Avoid duplicate job_ids
                    if job_id not in seen_job_ids:
                        seen_job_ids.add(job_id)
                        result["base_branch"] = base_branch
                        failed_attempts.append(result)
            except Exception as exc:
                workflow_name, attempt_num = futures[future]
                print(f"Error processing workflow {workflow_name} attempt {attempt_num}: {exc}")

    return failed_attempts

def display_attempt(attempt):
    markdown = []
    markdown.append(f"### {attempt['job_name']}")
    markdown.append(f"**Attempt** {attempt['attempt_number']} [View logs]({attempt['log_url']})")

    if attempt["failure_lines"]:
        markdown.append("```")
        for line in attempt["failure_lines"]:
            markdown.append(line)
        markdown.append("```")
    else:
        markdown.append("\n*No specific failure lines found.*")

    markdown.append("\n---\n")
    return "\n".join(markdown)

def main():
    parser = argparse.ArgumentParser(description="Lists failed GitHub Actions workflow attempts for a specific Pull Request")
    parser.add_argument("pr_number", type=int, help="PR number to analyze")
    parser.add_argument("--repo", help="Repository in owner/name format (can also be set via REPOSITORY environment variable)")
    parser.add_argument("--max-attempts", type=int, help="Maximum number of CI attempts (can also be set via MAX_CI_ATTEMPTS environment variable)")

    args = parser.parse_args()

    repository = args.repo or os.environ.get("REPOSITORY")
    if not repository:
        print("Error: Repository not specified. Use --repo option or set the REPOSITORY environment variable")
        sys.exit(1)

    try:
        repo_owner, repo_name = repository.split('/')
    except ValueError:
        print("Error: Repository must be in the format 'owner/name'")
        sys.exit(1)

    max_ci_attempts = args.max_attempts or os.environ.get("MAX_CI_ATTEMPTS")
    if max_ci_attempts:
        try:
            max_ci_attempts = int(max_ci_attempts)
        except ValueError:
            max_ci_attempts = None

    markdown_output = []
    markdown_output.append(f"# CI Failures")

    failed_attempts = get_failed_workflows(repo_owner, repo_name, args.pr_number)

    if not failed_attempts:
        markdown_output.append("#### No failed workflow attempts found.")
        print("\n".join(markdown_output))
        return

    failed_attempts.sort(key=lambda x: (x["workflow_name"], x["attempt_number"]))

    # Calculate unique attempt numbers that had failures
    unique_failed_attempts = sorted(set(attempt["attempt_number"] for attempt in failed_attempts))
    num_failed_attempts = len(unique_failed_attempts)
    num_failed_jobs = len(failed_attempts)

    if max_ci_attempts:
        summary = f"**{num_failed_attempts}/{max_ci_attempts}** CI run attempts had failures"
    else:
        summary = f"**{num_failed_attempts}** CI run {'attempt' if num_failed_attempts == 1 else 'attempts'} had failures"

    summary += f" with **{num_failed_jobs}** failed {'job' if num_failed_jobs == 1 else 'jobs'} total"

    markdown_output.append(summary)
    markdown_output.append("\n---\n")

    for attempt in failed_attempts:
        markdown_output.append(display_attempt(attempt))

    print("\n".join(markdown_output))

if __name__ == "__main__":
    main()