#!/usr/bin/env bash

results_dir="${RESULTS_DIR:-/tmp/results}"
junit_report_file="${results_dir}/report.xml"

# saveResults prepares the results for handoff to the Sonobuoy worker.
# See: https://github.com/vmware-tanzu/sonobuoy/blob/master/site/docs/master/plugins.md
saveResults() {
  # Signal to the worker that we are done and where to find the results.
  printf ${junit_report_file} >"${results_dir}/done"
}

# Ensure that we tell the Sonobuoy worker we are done regardless of results.
trap saveResults EXIT

mkdir "${results_dir}" || true

pytest -v -s /src/rancher-validation/tests/v3_api/test_configmaps.py --junit-xml=report.xml
cp  report.xml "${results_dir}/report.xml"