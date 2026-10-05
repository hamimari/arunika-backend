#!/usr/bin/env bash
# Production deploy, run on the VPS by .github/workflows/deploy.yml in this
# repo and in arunika-backoffice. The CI key is installed for the `deploy` user
# as a forced command, so this script is the only thing that key can run:
#
#   command="/opt/arunika/arunika-backend/deploy/deploy.sh",no-pty,no-port-forwarding,no-agent-forwarding,no-X11-forwarding ssh-ed25519 AAAA… ci-deploy
#
# The request arrives in SSH_ORIGINAL_COMMAND (or as arguments when run by
# hand) and must be exactly `backend <sha>` or `backoffice <sha>`, with a full
# 40-character sha that is already on origin/master.
#
# A backend deploy checks out a new copy of this file while it is running.
# Everything lives inside main(), and the last line calls it and exits, so
# bash has parsed the whole script before the checkout can change it.

main() {
  set -euo pipefail

  local root=${ARUNIKA_ROOT:-/opt/arunika}
  local request=${SSH_ORIGINAL_COMMAND:-$*}
  local re='^(backend|backoffice) ([0-9a-f]{40})$'
  if [[ ! $request =~ $re ]]; then
    echo "refused: expected 'backend <sha>' or 'backoffice <sha>'" >&2
    exit 64
  fi
  local service=${BASH_REMATCH[1]} sha=${BASH_REMATCH[2]}

  # GitHub's concurrency groups are per repo; this lock also keeps a backend
  # and a backoffice deploy from overlapping.
  exec 9>>"$root/.deploy.lock"
  if ! flock -w 900 9; then
    echo "another deploy still holds the lock after 15 minutes" >&2
    exit 75
  fi

  local repo container
  case $service in
    backend) repo=$root/arunika-backend container=arunika_app ;;
    backoffice) repo=$root/arunika-backoffice container=arunika_backoffice ;;
  esac

  git -C "$repo" fetch --quiet origin master
  if ! git -C "$repo" merge-base --is-ancestor "$sha" origin/master 2>/dev/null; then
    echo "refused: $sha is not on origin/master of $service" >&2
    exit 65
  fi

  local old
  old=$(git -C "$repo" rev-parse HEAD)
  echo "deploying $service ${old:0:12} -> ${sha:0:12}"
  git -C "$repo" checkout --quiet --detach "$sha"

  local status=ok
  cd "$root/arunika-backend"
  local compose=(docker compose -f docker-compose.yml -f docker-compose.prod.yml)
  case $service in
    # `app` depends on flyway completing, so pending migrations run first.
    # postgres and redis are left alone unless their config changed.
    backend) "${compose[@]}" up -d --build app || status=failed ;;
    backoffice) "${compose[@]}" up -d --build --no-deps backoffice || status=failed ;;
  esac
  if [[ $status == ok ]]; then
    wait_ready "$container" || status=failed
  fi

  printf '%s %s %s -> %s %s\n' "$(date -u +%FT%TZ)" "$service" "$old" "$sha" "$status" \
    >>"$root/deploy-history.log"

  docker image prune -f >/dev/null || true
  docker builder prune -f --filter until=168h >/dev/null || true

  if [[ $status != ok ]]; then
    echo "deploy of $service $sha failed; recent logs:" >&2
    docker logs --tail 50 "$container" >&2 || true
    exit 1
  fi
  echo "deployed $service $sha"
}

# Waits up to 3 minutes for the container's health check to pass, or, for a
# container without one (the backoffice), for it to be running.
wait_ready() {
  local container=$1 state
  for _ in $(seq 1 60); do
    state=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)
    case $state in
      healthy | running) return 0 ;;
      unhealthy | exited | dead) break ;;
    esac
    sleep 3
  done
  echo "$container did not become ready (last state: ${state:-unknown})" >&2
  return 1
}

main "$@"
exit
