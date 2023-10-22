#!/usr/bin/env bash
#
# This script configures a fresh instance of HASS running
# as part of the associated Docker Compose environment.

set -e

HAURL="http://localhost:8123"

HANAME="admin"
HAUSER="${HANAME}"
HAPASSWORD="${HANAME}"

# Create temporary directory which is deleted on exit.
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TMPDIR=`mktemp -d -p "$DIR"`
function cleanup {
  rm -rf "${TMPDIR}"
}
trap cleanup EXIT

curl "${HAURL}/api/onboarding/users" \
  --silent \
  -H 'Content-Type: text/plain;charset=UTF-8' \
  --data-binary "{\"client_id\":\"${HAURL}\",\"name\":\"${HANAME}\",\"username\":\"${HAUSER}\",\"password\":\"${HAPASSWORD}\",\"language\":\"en\"}" \
  --output - \
  | jq -r '.auth_code' > "${TMPDIR}/auth_code"

curl "${HAURL}/auth/token" \
  --silent \
  -X 'POST' \
  -F "client_id=${HAURL}" \
  -F "code=$(cat "${TMPDIR}/auth_code")" \
  -F 'grant_type=authorization_code' \
  --output - \
  | jq -r '.access_token' > "${TMPDIR}/access_token"

curl "${HAURL}/api/onboarding/core_config" \
  --silent \
  -X 'POST' \
  -H "Authorization: Bearer $(cat "${TMPDIR}/access_token")" \
  -o /dev/null

curl "${HAURL}/api/onboarding/analytics" \
  --silent \
  -X 'POST' \
  -H "Authorization: Bearer $(cat "${TMPDIR}/access_token")" \
  -o /dev/null

curl "${HAURL}/api/onboarding/integration" \
  --silent \
  -H 'Content-Type: application/json;charset=UTF-8' \
  -H "Authorization: Bearer $(cat "${TMPDIR}/access_token")" \
  --data-binary "{\"client_id\":\"${HAURL}\",\"redirect_uri\":\"${HAURL}/?auth_callback=1\"}" \
  -o /dev/null

# Configure MQTT integration.

curl "${HAURL}/api/config/config_entries/flow" \
  --silent \
  -X 'POST' \
  -H 'Content-Type: application/json;charset=UTF-8' \
  -H "Authorization: Bearer $(cat "${TMPDIR}/access_token")" \
  --data-binary '{"handler":"mqtt","show_advanced_options":false}' \
  --output - \
  | jq -r '.flow_id' > "${TMPDIR}/flow_id"

curl "${HAURL}/api/config/config_entries/flow/$(cat "${TMPDIR}/flow_id")" \
  --silent \
  -X 'POST' \
  -H 'Content-Type: application/json;charset=UTF-8' \
  -H "Authorization: Bearer $(cat "${TMPDIR}/access_token")" \
  --data-binary '{"broker":"mosquitto","port":1883}' \
  -o /dev/null

echo "Created user '${HAUSER}'"
