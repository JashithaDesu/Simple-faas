#!/bin/bash
# Cold-start vs warm-request latency benchmark for mini-faas.
#
# Design note: cold runs force zero replicas by deleting the backing
# Deployment directly, rather than waiting out the real idle-timeout
# clock between every iteration. This exercises the identical gateway
# cold-start code path (Cold phase -> trigger -> wait for Warm) without
# a 60+ second wait per sample. The idle-timeout mechanism itself was
# already verified separately (see session log); this script measures
# cold-start latency magnitude, not the idle-timeout timer's correctness.
#
# Usage: ./benchmark.sh [gateway_url] [function_name] [iterations]
# Requires: the gateway port-forward already running (default assumes
# localhost:9998 -> mini-faas-gateway service, port 8080).

set -euo pipefail

GATEWAY_URL="${1:-localhost:9998}"
FUNCTION="${2:-hello-function}"
ITERATIONS="${3:-5}"
OUTFILE="benchmark-results.csv"

echo "type,iteration,latency_seconds" > "$OUTFILE"

wait_for_pod_gone() {
	local timeout=20
	local elapsed=0
	while kubectl get pods -l "faas-function=${FUNCTION}" --no-headers 2>/dev/null | grep -q .; do
		sleep 1
		elapsed=$((elapsed + 1))
		if [ "$elapsed" -ge "$timeout" ]; then
			echo "WARNING: pod did not disappear within ${timeout}s" >&2
			break
		fi
	done
}

wait_for_pod_ready() {
	local timeout=20
	local elapsed=0
	while ! kubectl get pods -l "faas-function=${FUNCTION}" --no-headers 2>/dev/null | grep -q "1/1.*Running"; do
		sleep 1
		elapsed=$((elapsed + 1))
		if [ "$elapsed" -ge "$timeout" ]; then
			echo "WARNING: pod did not reach Running within ${timeout}s" >&2
			break
		fi
	done
}

echo "=== Cold-start runs (${ITERATIONS} iterations) ==="
for i in $(seq 1 "$ITERATIONS"); do
		echo "--- cold run $i: forcing cold state ---"
	kubectl patch function "$FUNCTION" --type=merge -p '{"spec":{"minReplicas":0,"triggeredReplicas":null}}' >/dev/null 2>&1
	kubectl delete deployment "$FUNCTION" --ignore-not-found=true >/dev/null 2>&1 || true
	wait_for_pod_gone

	latency=$(curl -s -o /dev/null -w "%{time_total}" "http://${GATEWAY_URL}/invoke/${FUNCTION}")
	echo "cold run $i: ${latency}s"
	echo "cold,${i},${latency}" >> "$OUTFILE"

	sleep 2
done

echo ""
echo "=== Warm-request runs (${ITERATIONS} iterations) ==="
echo "--- ensuring function is warm before measuring ---"
curl -s -o /dev/null "http://${GATEWAY_URL}/invoke/${FUNCTION}"
wait_for_pod_ready
sleep 1

for i in $(seq 1 "$ITERATIONS"); do
	latency=$(curl -s -o /dev/null -w "%{time_total}" "http://${GATEWAY_URL}/invoke/${FUNCTION}")
	echo "warm run $i: ${latency}s"
	echo "warm,${i},${latency}" >> "$OUTFILE"
	sleep 1
done

echo ""
echo "=== Summary ==="
echo "Cold-start latencies:"
grep "^cold," "$OUTFILE" | cut -d, -f3
echo "Warm-request latencies:"
grep "^warm," "$OUTFILE" | cut -d, -f3
echo ""
echo "Full results written to $OUTFILE"
