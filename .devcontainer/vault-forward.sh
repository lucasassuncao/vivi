#!/usr/bin/env bash
# Puts the host's Vault on this container's loopback: the sibling container
# publishes 8200 on the host, and the integration suite refuses any VAULT_ADDR
# that is not loopback, so socat makes the name true instead.
set -euo pipefail

PORT=${VAULT_FORWARD_PORT:-8200}

# Probed, not assumed: socat accepts a connection before trying the other end,
# so a wrong target is silent. The gateway goes before host.docker.internal,
# which on Docker Desktop does not reach a published port.
candidates=()
[ -n "${VAULT_FORWARD_TARGET:-}" ] && candidates+=("$VAULT_FORWARD_TARGET")
gateway=$(ip route 2>/dev/null | awk '/^default/ {print $3; exit}' || true)
[ -n "$gateway" ] && candidates+=("$gateway")
candidates+=("host.docker.internal")

# An open socket does not prove the forward works, so ask for a real answer,
# or re-running this would keep the broken forwarder it was run to replace.
if pgrep -f "LISTEN:${PORT},bind=127.0.0.1" >/dev/null 2>&1; then
  if curl -fsS -m 3 -o /dev/null "http://127.0.0.1:${PORT}/v1/sys/health" 2>/dev/null; then
    echo "vault-forward: already listening on 127.0.0.1:${PORT}"
    exit 0
  fi
  echo "vault-forward: forwarder on 127.0.0.1:${PORT} is not answering, replacing it"
  # By PID of a real socat: `pkill -f` also matches a shell invoked with these
  # arguments.
  for pid in $(pgrep -x socat); do
    grep -qa "LISTEN:${PORT},bind=127.0.0.1" "/proc/$pid/cmdline" 2>/dev/null &&
      kill "$pid" 2>/dev/null || true
  done
  sleep 1
fi

target=""
for candidate in "${candidates[@]}"; do
  if timeout 2 bash -c "exec 3<>/dev/tcp/$candidate/$PORT" 2>/dev/null; then
    target=$candidate
    break
  fi
done

# Nothing answering is the normal state before `make vault-up`; socat binds
# anyway and picks the traffic up later.
if [ -z "$target" ]; then
  target=${candidates[0]}
  echo "vault-forward: nothing on :${PORT} yet, assuming ${target}"
fi

# setsid because the tooling reaps this hook's process group, and nohup only
# ignores SIGHUP. TCP4 skips an IPv6 record that routes nowhere; connect-timeout
# keeps a dead target from hanging the caller and leaking a child per attempt.
setsid socat \
  "TCP4-LISTEN:${PORT},bind=127.0.0.1,fork,reuseaddr" \
  "TCP4:${target}:${PORT},connect-timeout=5" \
  </dev/null >/tmp/vault-forward.log 2>&1 &

echo "vault-forward: 127.0.0.1:${PORT} -> ${target}:${PORT} (log: /tmp/vault-forward.log)"
