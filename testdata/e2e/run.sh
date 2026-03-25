#!/usr/bin/env bash
#
# Run e2e tests locally against Azure infrastructure.
# Assumes Azure CLI is already authenticated (az login).
#
# Usage:
#   ./testdata/e2e/run.sh --build                                  # Build from source, run all tests
#   ./testdata/e2e/run.sh --version 0.4.0                          # Download released version
#   ./testdata/e2e/run.sh --build --plan-only                      # Skip apply/destroy (faster)
#   ./testdata/e2e/run.sh --build --evidence                       # Enable evidence signing
#   ./testdata/e2e/run.sh --build -t route-table -t blast-radius   # Run specific tests only
#   ./testdata/e2e/run.sh --build --capture                        # Run + save plan JSON fixtures
#   ./testdata/e2e/run.sh --build --fixtures                       # Classify committed fixtures (no Azure)
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
E2E_DIR="$REPO_ROOT/testdata/e2e"
export TFCLASSIFY_PLUGIN_DIR="${TFCLASSIFY_PLUGIN_DIR:-/tmp/tfclassify-plugins}"
PLUGIN_DIR="$TFCLASSIFY_PLUGIN_DIR"
LOG_DIR="$REPO_ROOT/testdata/e2e/.logs"

# Scenarios that always run plan-only (mirrors CI matrix).
# custom-role-cross-reference: requires Microsoft.Authorization/roleDefinitions/write
# evidence-signing: only needs a plan to validate evidence generation
ALWAYS_PLAN_ONLY=("custom-role-cross-reference" "evidence-signing")

# Scenarios that run with evidence verification when --evidence is set.
EVIDENCE_SCENARIOS=("evidence-signing")

# --- Defaults ---
TFCLASSIFY_BIN=""
BUILD=false
VERSION=""
EVIDENCE=false
PLAN_ONLY=false
CAPTURE=false
FIXTURES=false
TESTS=()

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
DIM='\033[2m'
BOLD='\033[1m'
NC='\033[0m'

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Run e2e tests locally against Azure infrastructure.
Assumes you are already signed in with the Azure CLI.

Options:
  --build              Build tfclassify and plugin from source
  --version VERSION    Download released CLI at VERSION (e.g., 0.4.0)
  --evidence           Enable evidence signing and verification
  --plan-only          Skip apply/destroy for all scenarios (faster iteration)
  --capture            Save plan JSON to fixtures/ after generation (for fixture-based tests)
  --fixtures           Classify committed fixtures instead of running Terraform (no Azure needed)
  -t, --test NAME      Run only the named scenario (repeatable)
  -h, --help           Show this help message

Scenarios that always run plan-only regardless of --plan-only flag:
  custom-role-cross-reference   (requires roleDefinitions/write)
  evidence-signing              (only needs plan for verification)

Examples:
  $(basename "$0") --build
  $(basename "$0") --build --plan-only
  $(basename "$0") --build -t route-table -t blast-radius
  $(basename "$0") --build --evidence
  $(basename "$0") --version 0.4.0 --plan-only -t blast-radius
  $(basename "$0") --build --capture                        # refresh fixtures
  $(basename "$0") --build --fixtures                       # fast, no Azure
EOF
  exit 0
}

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --build)     BUILD=true; shift ;;
    --version)   VERSION="$2"; shift 2 ;;
    --evidence)  EVIDENCE=true; shift ;;
    --plan-only) PLAN_ONLY=true; shift ;;
    --capture)   CAPTURE=true; shift ;;
    --fixtures)  FIXTURES=true; shift ;;
    -t|--test)   TESTS+=("$2"); shift 2 ;;
    -h|--help)   usage ;;
    *)           echo -e "${RED}Unknown option: $1${NC}" >&2; echo; usage ;;
  esac
done

if [[ "$FIXTURES" == true && -n "$VERSION" ]]; then
  echo -e "${RED}Error: --fixtures and --version are mutually exclusive${NC}" >&2
  exit 1
fi
if [[ "$FIXTURES" == true && "$BUILD" == false ]]; then
  echo -e "${RED}Error: --fixtures requires --build${NC}" >&2
  exit 1
fi
if [[ "$BUILD" == false && -z "$VERSION" ]]; then
  echo -e "${RED}Error: specify --build or --version VERSION${NC}" >&2
  exit 1
fi
if [[ "$BUILD" == true && -n "$VERSION" ]]; then
  echo -e "${RED}Error: --build and --version are mutually exclusive${NC}" >&2
  exit 1
fi

# --- Build from source ---
if [[ "$BUILD" == true ]]; then
  echo -e "${CYAN}Building tfclassify from source...${NC}"
  (cd "$REPO_ROOT" && go build -o "$REPO_ROOT/bin/tfclassify" ./cmd/tfclassify)
  TFCLASSIFY_BIN="$REPO_ROOT/bin/tfclassify"

  echo -e "${CYAN}Building azurerm plugin...${NC}"
  mkdir -p "$PLUGIN_DIR"
  (cd "$REPO_ROOT" && go build -o "$PLUGIN_DIR/tfclassify-plugin-azurerm" ./plugins/azurerm)
fi

# --- Download released version ---
if [[ -n "$VERSION" ]]; then
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"
  case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
  esac

  ASSET="tfclassify_${VERSION}_${OS}_${ARCH}.zip"
  TAG="tfclassify-v${VERSION}"
  echo -e "${CYAN}Downloading tfclassify ${VERSION} (${OS}/${ARCH})...${NC}"
  mkdir -p "$REPO_ROOT/bin"
  gh release download "$TAG" --pattern "$ASSET" --dir "$REPO_ROOT/bin" --clobber
  unzip -o "$REPO_ROOT/bin/$ASSET" -d "$REPO_ROOT/bin" >/dev/null
  rm -f "$REPO_ROOT/bin/$ASSET"
  chmod +x "$REPO_ROOT/bin/tfclassify"
  TFCLASSIFY_BIN="$REPO_ROOT/bin/tfclassify"
fi

if [[ ! -x "$TFCLASSIFY_BIN" ]]; then
  echo -e "${RED}Error: $TFCLASSIFY_BIN not found or not executable${NC}" >&2
  exit 1
fi

echo -e "${CYAN}Binary:${NC}    $TFCLASSIFY_BIN"
"$TFCLASSIFY_BIN" --version 2>/dev/null || true
if [[ "$FIXTURES" == false ]]; then
  echo -e "${CYAN}Terraform:${NC} $(terraform --version -json 2>/dev/null | jq -r .terraform_version 2>/dev/null || terraform --version | head -1)"
fi
echo ""

# --- Discover scenarios ---
if [[ ${#TESTS[@]} -gt 0 ]]; then
  SCENARIOS=("${TESTS[@]}")
  for t in "${SCENARIOS[@]}"; do
    if [[ ! -f "$E2E_DIR/$t/expected.json" ]]; then
      echo -e "${RED}Error: scenario '$t' not found in $E2E_DIR${NC}" >&2
      exit 1
    fi
  done
else
  SCENARIOS=()
  for dir in "$E2E_DIR"/*/; do
    name="$(basename "$dir")"
    if [[ -f "$dir/expected.json" ]]; then
      SCENARIOS+=("$name")
    fi
  done
fi

echo -e "${BOLD}Running ${#SCENARIOS[@]} scenario(s):${NC} ${SCENARIOS[*]}"
if [[ "$FIXTURES" == true ]]; then
  echo -e "${YELLOW}Fixture mode: classifying committed plan fixtures (no Azure)${NC}"
elif [[ "$PLAN_ONLY" == true ]]; then
  echo -e "${YELLOW}Plan-only mode: skipping apply/destroy for all scenarios${NC}"
fi
if [[ "$CAPTURE" == true ]]; then
  echo -e "${YELLOW}Capture mode: saving plan JSON to fixtures/${NC}"
fi
echo ""

# --- Prepare log directory ---
rm -rf "$LOG_DIR"
mkdir -p "$LOG_DIR"

# --- Evidence key pair ---
EVIDENCE_PRIVATE_KEY=""
EVIDENCE_PUBLIC_KEY=""
if [[ "$EVIDENCE" == true ]]; then
  EVIDENCE_PRIVATE_KEY="$(mktemp /tmp/evidence-private-XXXXXX.pem)"
  EVIDENCE_PUBLIC_KEY="$(mktemp /tmp/evidence-public-XXXXXX.pem)"
  openssl genpkey -algorithm Ed25519 -out "$EVIDENCE_PRIVATE_KEY" 2>/dev/null
  openssl pkey -in "$EVIDENCE_PRIVATE_KEY" -pubout -out "$EVIDENCE_PUBLIC_KEY" 2>/dev/null
  echo -e "${CYAN}Generated Ed25519 evidence key pair${NC}"
  echo ""
fi

# --- Helpers ---
in_array() {
  local needle="$1"; shift
  for item in "$@"; do
    [[ "$item" == "$needle" ]] && return 0
  done
  return 1
}

cleanup_scenario() {
  local dir="$1"
  rm -f "$dir/create.tfplan" "$dir/create.json" \
        "$dir/create-result.json" "$dir/create-result-binary.json" \
        "$dir/destroy.tfplan" "$dir/destroy.json" \
        "$dir/destroy-result.json" "$dir/destroy-result-binary.json"
  rm -rf "$dir/.terraform"
  rm -f "$dir/.terraform.lock.hcl" "$dir/.terraform.tfstate.lock.info" \
        "$dir/terraform.tfstate" "$dir/terraform.tfstate.backup"
}

cleanup_all() {
  for scenario in "${SCENARIOS[@]}"; do
    cleanup_scenario "$E2E_DIR/$scenario"
  done
}
if [[ "$FIXTURES" == false ]]; then
  trap cleanup_all EXIT
fi

# --- Run a single scenario ---
# Terraform output goes to $LOG_DIR/$name/terraform.log
# Test results (PASS/FAIL/SKIP) go to $LOG_DIR/$name/results.log
run_scenario() {
  local name="$1"
  local scenario_dir="$E2E_DIR/$name"
  local scenario_log_dir="$LOG_DIR/$name"
  local tf_log="$scenario_log_dir/terraform.log"
  local results_log="$scenario_log_dir/results.log"
  local is_plan_only="$PLAN_ONLY"
  local use_evidence=false
  local failed=false
  local applied=false

  mkdir -p "$scenario_log_dir"

  # Force plan-only for scenarios that require it
  if in_array "$name" "${ALWAYS_PLAN_ONLY[@]}"; then
    is_plan_only=true
  fi

  # Enable evidence for evidence scenarios when --evidence is set
  if [[ "$EVIDENCE" == true ]] && in_array "$name" "${EVIDENCE_SCENARIOS[@]}"; then
    use_evidence=true
  fi

  # Export signing key for evidence scenarios
  if [[ "$use_evidence" == true ]]; then
    export TFCLASSIFY_SIGNING_KEY="$EVIDENCE_PRIVATE_KEY"
  fi

  # Validate config
  if ! "$TFCLASSIFY_BIN" validate -c "$scenario_dir/.tfclassify.hcl" >> "$tf_log" 2>&1; then
    echo "FAIL: config validation (see terraform.log)" >> "$results_log"
    echo "FAIL" > "$scenario_log_dir/status"
    return
  fi

  # Terraform init
  if ! terraform -chdir="$scenario_dir" init -input=false >> "$tf_log" 2>&1; then
    echo "FAIL: terraform init (see terraform.log)" >> "$results_log"
    echo "FAIL" > "$scenario_log_dir/status"
    return
  fi

  # ---- Create phase ----
  if ! terraform -chdir="$scenario_dir" plan \
         -var-file=../e2e.tfvars -out=create.tfplan -input=false >> "$tf_log" 2>&1; then
    echo "FAIL: terraform plan (create) (see terraform.log)" >> "$results_log"
    echo "FAIL" > "$scenario_log_dir/status"
    cleanup_scenario "$scenario_dir"
    return
  fi
  terraform -chdir="$scenario_dir" show -json create.tfplan \
    > "$scenario_dir/create.json" 2>> "$tf_log"

  # Capture fixture if requested
  if [[ "$CAPTURE" == true ]]; then
    mkdir -p "$scenario_dir/fixtures"
    cp "$scenario_dir/create.json" "$scenario_dir/fixtures/create.json"
  fi

  local expected_create
  expected_create="$(jq -r '.create.exit_code' "$scenario_dir/expected.json")"

  # Classify create (JSON)
  local evidence_flag=""
  if [[ "$use_evidence" == true ]]; then
    evidence_flag="--evidence-file $scenario_log_dir/evidence-create-json.json"
  fi

  set +e
  # shellcheck disable=SC2086
  "$TFCLASSIFY_BIN" \
    -p "$scenario_dir/create.json" \
    -c "$scenario_dir/.tfclassify.hcl" \
    -o json --detailed-exitcode $evidence_flag \
    > "$scenario_dir/create-result.json" 2>> "$tf_log"
  local actual_create=$?
  set -e

  if [[ "$actual_create" != "$expected_create" ]]; then
    echo "FAIL: create (JSON) expected exit $expected_create, got $actual_create" >> "$results_log"
    failed=true
  else
    echo "PASS: create (JSON) exit code $actual_create" >> "$results_log"
  fi

  # Verify evidence (create JSON)
  if [[ "$use_evidence" == true && -f "$scenario_log_dir/evidence-create-json.json" ]]; then
    if "$TFCLASSIFY_BIN" verify \
         --evidence-file "$scenario_log_dir/evidence-create-json.json" \
         --public-key "$EVIDENCE_PUBLIC_KEY" >> "$tf_log" 2>&1; then
      echo "PASS: evidence verification (create JSON)" >> "$results_log"
    else
      echo "FAIL: evidence verification (create JSON)" >> "$results_log"
      failed=true
    fi
  fi

  # Classify create (SARIF)
  set +e
  "$TFCLASSIFY_BIN" \
    -p "$scenario_dir/create.json" \
    -c "$scenario_dir/.tfclassify.hcl" \
    -o sarif --detailed-exitcode > /dev/null 2>> "$tf_log"
  local actual_create_sarif=$?
  set -e

  if [[ "$actual_create_sarif" != "$expected_create" ]]; then
    echo "FAIL: create (SARIF) expected exit $expected_create, got $actual_create_sarif" >> "$results_log"
    failed=true
  else
    echo "PASS: create (SARIF) exit code $actual_create_sarif" >> "$results_log"
  fi

  # Classify create (binary plan)
  evidence_flag=""
  if [[ "$use_evidence" == true ]]; then
    evidence_flag="--evidence-file $scenario_log_dir/evidence-create-binary.json"
  fi

  set +e
  # shellcheck disable=SC2086
  "$TFCLASSIFY_BIN" \
    -p "$scenario_dir/create.tfplan" \
    -c "$scenario_dir/.tfclassify.hcl" \
    -o json --detailed-exitcode $evidence_flag \
    > "$scenario_dir/create-result-binary.json" 2>> "$tf_log"
  local actual_create_bin=$?
  set -e

  if [[ "$actual_create_bin" != "$expected_create" ]]; then
    echo "FAIL: create (binary) expected exit $expected_create, got $actual_create_bin" >> "$results_log"
    failed=true
  else
    echo "PASS: create (binary) exit code $actual_create_bin" >> "$results_log"
  fi

  # Verify evidence (create binary)
  if [[ "$use_evidence" == true && -f "$scenario_log_dir/evidence-create-binary.json" ]]; then
    if "$TFCLASSIFY_BIN" verify \
         --evidence-file "$scenario_log_dir/evidence-create-binary.json" \
         --public-key "$EVIDENCE_PUBLIC_KEY" >> "$tf_log" 2>&1; then
      echo "PASS: evidence verification (create binary)" >> "$results_log"
    else
      echo "FAIL: evidence verification (create binary)" >> "$results_log"
      failed=true
    fi
  fi

  # ---- Apply + Destroy phase ----
  if [[ "$is_plan_only" == true ]]; then
    echo "SKIP: apply/destroy (plan-only)" >> "$results_log"
  else
    if ! terraform -chdir="$scenario_dir" apply -auto-approve create.tfplan >> "$tf_log" 2>&1; then
      echo "FAIL: terraform apply (see terraform.log)" >> "$results_log"
      terraform -chdir="$scenario_dir" destroy -auto-approve -var-file=../e2e.tfvars >> "$tf_log" 2>&1 || true
      echo "FAIL" > "$scenario_log_dir/status"
      cleanup_scenario "$scenario_dir"
      return
    fi
    applied=true

    if ! terraform -chdir="$scenario_dir" plan -destroy \
           -var-file=../e2e.tfvars -out=destroy.tfplan -input=false >> "$tf_log" 2>&1; then
      echo "FAIL: terraform plan (destroy) (see terraform.log)" >> "$results_log"
      terraform -chdir="$scenario_dir" destroy -auto-approve -var-file=../e2e.tfvars >> "$tf_log" 2>&1 || true
      echo "FAIL" > "$scenario_log_dir/status"
      cleanup_scenario "$scenario_dir"
      return
    fi
    terraform -chdir="$scenario_dir" show -json destroy.tfplan \
      > "$scenario_dir/destroy.json" 2>> "$tf_log"

    # Capture fixture if requested
    if [[ "$CAPTURE" == true ]]; then
      mkdir -p "$scenario_dir/fixtures"
      cp "$scenario_dir/destroy.json" "$scenario_dir/fixtures/destroy.json"
    fi

    local expected_destroy
    expected_destroy="$(jq -r '.destroy.exit_code' "$scenario_dir/expected.json")"

    # Classify destroy (JSON)
    evidence_flag=""
    if [[ "$use_evidence" == true ]]; then
      evidence_flag="--evidence-file $scenario_log_dir/evidence-destroy-json.json"
    fi

    set +e
    # shellcheck disable=SC2086
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/destroy.json" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o json --detailed-exitcode $evidence_flag \
      > "$scenario_dir/destroy-result.json" 2>> "$tf_log"
    local actual_destroy=$?
    set -e

    if [[ "$actual_destroy" != "$expected_destroy" ]]; then
      echo "FAIL: destroy (JSON) expected exit $expected_destroy, got $actual_destroy" >> "$results_log"
      failed=true
    else
      echo "PASS: destroy (JSON) exit code $actual_destroy" >> "$results_log"
    fi

    # Classify destroy (SARIF)
    set +e
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/destroy.json" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o sarif --detailed-exitcode > /dev/null 2>> "$tf_log"
    local actual_destroy_sarif=$?
    set -e

    if [[ "$actual_destroy_sarif" != "$expected_destroy" ]]; then
      echo "FAIL: destroy (SARIF) expected exit $expected_destroy, got $actual_destroy_sarif" >> "$results_log"
      failed=true
    else
      echo "PASS: destroy (SARIF) exit code $actual_destroy_sarif" >> "$results_log"
    fi

    # Classify destroy (binary plan)
    evidence_flag=""
    if [[ "$use_evidence" == true ]]; then
      evidence_flag="--evidence-file $scenario_log_dir/evidence-destroy-binary.json"
    fi

    set +e
    # shellcheck disable=SC2086
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/destroy.tfplan" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o json --detailed-exitcode $evidence_flag \
      > "$scenario_dir/destroy-result-binary.json" 2>> "$tf_log"
    local actual_destroy_bin=$?
    set -e

    if [[ "$actual_destroy_bin" != "$expected_destroy" ]]; then
      echo "FAIL: destroy (binary) expected exit $expected_destroy, got $actual_destroy_bin" >> "$results_log"
      failed=true
    else
      echo "PASS: destroy (binary) exit code $actual_destroy_bin" >> "$results_log"
    fi

    # Verify evidence (destroy)
    if [[ "$use_evidence" == true ]]; then
      if [[ -f "$scenario_log_dir/evidence-destroy-json.json" ]]; then
        if "$TFCLASSIFY_BIN" verify \
             --evidence-file "$scenario_log_dir/evidence-destroy-json.json" \
             --public-key "$EVIDENCE_PUBLIC_KEY" >> "$tf_log" 2>&1; then
          echo "PASS: evidence verification (destroy JSON)" >> "$results_log"
        else
          echo "FAIL: evidence verification (destroy JSON)" >> "$results_log"
          failed=true
        fi
      fi
      if [[ -f "$scenario_log_dir/evidence-destroy-binary.json" ]]; then
        if "$TFCLASSIFY_BIN" verify \
             --evidence-file "$scenario_log_dir/evidence-destroy-binary.json" \
             --public-key "$EVIDENCE_PUBLIC_KEY" >> "$tf_log" 2>&1; then
          echo "PASS: evidence verification (destroy binary)" >> "$results_log"
        else
          echo "FAIL: evidence verification (destroy binary)" >> "$results_log"
          failed=true
        fi
      fi
    fi

    # Cleanup infrastructure
    terraform -chdir="$scenario_dir" destroy -auto-approve -var-file=../e2e.tfvars >> "$tf_log" 2>&1 || true
  fi

  # Clean up generated plan files
  cleanup_scenario "$scenario_dir"

  if [[ "$failed" == true ]]; then
    echo "FAIL" > "$scenario_log_dir/status"
  else
    echo "PASS" > "$scenario_log_dir/status"
  fi
}

# --- Run fixture-based scenario (no Terraform/Azure) ---
run_fixture_scenario() {
  local name="$1"
  local scenario_dir="$E2E_DIR/$name"
  local scenario_log_dir="$LOG_DIR/$name"
  local results_log="$scenario_log_dir/results.log"
  local use_evidence=false
  local failed=false

  mkdir -p "$scenario_log_dir"

  if [[ ! -f "$scenario_dir/fixtures/create.json" ]]; then
    echo "SKIP: no fixtures (run --capture to generate)" >> "$results_log"
    echo "SKIP" > "$scenario_log_dir/status"
    return
  fi

  # Enable evidence for evidence scenarios when --evidence is set
  if [[ "$EVIDENCE" == true ]] && in_array "$name" "${EVIDENCE_SCENARIOS[@]}"; then
    use_evidence=true
    export TFCLASSIFY_SIGNING_KEY="$EVIDENCE_PRIVATE_KEY"
  fi

  # Validate config
  if ! "$TFCLASSIFY_BIN" validate -c "$scenario_dir/.tfclassify.hcl" >> /dev/null 2>&1; then
    echo "FAIL: config validation" >> "$results_log"
    echo "FAIL" > "$scenario_log_dir/status"
    return
  fi

  local expected_create
  expected_create="$(jq -r '.create.exit_code' "$scenario_dir/expected.json")"

  # Classify create (JSON)
  local evidence_flag=""
  if [[ "$use_evidence" == true ]]; then
    evidence_flag="--evidence-file $scenario_log_dir/evidence-create-json.json"
  fi

  set +e
  # shellcheck disable=SC2086
  "$TFCLASSIFY_BIN" \
    -p "$scenario_dir/fixtures/create.json" \
    -c "$scenario_dir/.tfclassify.hcl" \
    -o json --detailed-exitcode $evidence_flag \
    > /dev/null 2>&1
  local actual_create=$?
  set -e

  if [[ "$actual_create" != "$expected_create" ]]; then
    echo "FAIL: create (JSON) expected exit $expected_create, got $actual_create" >> "$results_log"
    failed=true
  else
    echo "PASS: create (JSON) exit code $actual_create" >> "$results_log"
  fi

  # Classify create (SARIF)
  set +e
  "$TFCLASSIFY_BIN" \
    -p "$scenario_dir/fixtures/create.json" \
    -c "$scenario_dir/.tfclassify.hcl" \
    -o sarif --detailed-exitcode > /dev/null 2>&1
  local actual_create_sarif=$?
  set -e

  if [[ "$actual_create_sarif" != "$expected_create" ]]; then
    echo "FAIL: create (SARIF) expected exit $expected_create, got $actual_create_sarif" >> "$results_log"
    failed=true
  else
    echo "PASS: create (SARIF) exit code $actual_create_sarif" >> "$results_log"
  fi

  # Verify evidence (create)
  if [[ "$use_evidence" == true && -f "$scenario_log_dir/evidence-create-json.json" ]]; then
    if "$TFCLASSIFY_BIN" verify \
         --evidence-file "$scenario_log_dir/evidence-create-json.json" \
         --public-key "$EVIDENCE_PUBLIC_KEY" > /dev/null 2>&1; then
      echo "PASS: evidence verification (create)" >> "$results_log"
    else
      echo "FAIL: evidence verification (create)" >> "$results_log"
      failed=true
    fi
  fi

  # Classify destroy fixture if it exists
  if [[ -f "$scenario_dir/fixtures/destroy.json" ]]; then
    local expected_destroy
    expected_destroy="$(jq -r '.destroy.exit_code' "$scenario_dir/expected.json")"

    set +e
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/fixtures/destroy.json" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o json --detailed-exitcode > /dev/null 2>&1
    local actual_destroy=$?
    set -e

    if [[ "$actual_destroy" != "$expected_destroy" ]]; then
      echo "FAIL: destroy (JSON) expected exit $expected_destroy, got $actual_destroy" >> "$results_log"
      failed=true
    else
      echo "PASS: destroy (JSON) exit code $actual_destroy" >> "$results_log"
    fi

    # Classify destroy (SARIF)
    set +e
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/fixtures/destroy.json" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o sarif --detailed-exitcode > /dev/null 2>&1
    local actual_destroy_sarif=$?
    set -e

    if [[ "$actual_destroy_sarif" != "$expected_destroy" ]]; then
      echo "FAIL: destroy (SARIF) expected exit $expected_destroy, got $actual_destroy_sarif" >> "$results_log"
      failed=true
    else
      echo "PASS: destroy (SARIF) exit code $actual_destroy_sarif" >> "$results_log"
    fi
  fi

  if [[ "$failed" == true ]]; then
    echo "FAIL" > "$scenario_log_dir/status"
  else
    echo "PASS" > "$scenario_log_dir/status"
  fi
}

# --- Install plugins for released versions ---
if [[ -n "$VERSION" ]]; then
  mkdir -p "$PLUGIN_DIR"
  for scenario_dir in "$E2E_DIR"/*/; do
    config="$scenario_dir/.tfclassify.hcl"
    if [[ -f "$config" ]] && grep -q 'plugin "' "$config"; then
      name="$(basename "$scenario_dir")"
      # Skip scenarios not in the test list (if filtered)
      if [[ ${#TESTS[@]} -gt 0 ]] && ! in_array "$name" "${TESTS[@]}"; then
        continue
      fi
      echo -e "  ${DIM}Installing plugins for $name...${NC}"
      "$TFCLASSIFY_BIN" init -c "$config" 2>&1 | sed "s/^/    /"
    fi
  done
  echo ""
fi

# --- Launch all scenarios in parallel ---
PIDS=()
PID_SCENARIOS=()
for scenario in "${SCENARIOS[@]}"; do
  if [[ "$FIXTURES" == true ]]; then
    run_fixture_scenario "$scenario" &
  else
    run_scenario "$scenario" &
  fi
  pid=$!
  PIDS+=($pid)
  PID_SCENARIOS+=("$pid:$scenario")
  echo -e "  ${CYAN}Started${NC}  $scenario (pid $pid)"
done

echo ""
echo -e "${BOLD}Waiting for ${#PIDS[@]} scenario(s)...${NC}"
echo ""

# Wait and report as each finishes
for entry in "${PID_SCENARIOS[@]}"; do
  pid="${entry%%:*}"
  scenario="${entry#*:}"
  if wait "$pid" 2>/dev/null; then
    : # process exited (status in file)
  fi
  status_file="$LOG_DIR/$scenario/status"
  if [[ -f "$status_file" && "$(cat "$status_file")" == "PASS" ]]; then
    echo -e "  ${GREEN}PASS${NC}  $scenario"
  else
    echo -e "  ${RED}FAIL${NC}  $scenario"
  fi
done

# --- Summary ---
echo ""
PASSED=0
FAILED=0
ERRORS=()
for scenario in "${SCENARIOS[@]}"; do
  status_file="$LOG_DIR/$scenario/status"
  if [[ -f "$status_file" && "$(cat "$status_file")" == "PASS" ]]; then
    PASSED=$((PASSED + 1))
  else
    FAILED=$((FAILED + 1))
    ERRORS+=("$scenario")
  fi
done

echo -e "${BOLD}Results: ${GREEN}$PASSED passed${NC}, ${RED}$FAILED failed${NC} (${#SCENARIOS[@]} total)"

# Print concise failure details + log paths
if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo ""
  for scenario in "${ERRORS[@]}"; do
    results_file="$LOG_DIR/$scenario/results.log"
    echo -e "${RED}FAIL${NC}  ${BOLD}$scenario${NC}"
    if [[ -f "$results_file" ]]; then
      while IFS= read -r line; do
        echo -e "        $line"
      done < "$results_file"
    else
      echo -e "        (no results — failed before classification)"
    fi
  done
  echo ""
  echo -e "${DIM}Logs: $LOG_DIR/<scenario>/{results.log,terraform.log}${NC}"
fi

# Cleanup evidence keys
if [[ -n "$EVIDENCE_PRIVATE_KEY" ]]; then
  rm -f "$EVIDENCE_PRIVATE_KEY" "$EVIDENCE_PUBLIC_KEY"
fi

if [[ $FAILED -gt 0 ]]; then
  exit 1
fi
