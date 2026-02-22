#!/usr/bin/env bash
#
# Run e2e tests locally against Azure infrastructure.
# Assumes Azure CLI is already authenticated (az login).
#
# Usage:
#   ./testdata/e2e/run.sh --build                                 # Build from source, run all tests
#   ./testdata/e2e/run.sh --binary bin/tfclassify                  # Use prebuilt binary
#   ./testdata/e2e/run.sh --build --plan-only                      # Skip apply/destroy (faster)
#   ./testdata/e2e/run.sh --build --evidence                       # Enable evidence signing
#   ./testdata/e2e/run.sh --build -t route-table -t blast-radius   # Run specific tests only
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
E2E_DIR="$REPO_ROOT/testdata/e2e"
PLUGIN_DIR="${TFCLASSIFY_PLUGIN_DIR:-/tmp/tfclassify-plugins}"
RESULTS_DIR="$(mktemp -d)"

# Scenarios that always run plan-only (mirrors CI matrix).
# custom-role-cross-reference: requires Microsoft.Authorization/roleDefinitions/write
# evidence-signing: only needs a plan to validate evidence generation
ALWAYS_PLAN_ONLY=("custom-role-cross-reference" "evidence-signing")

# Scenarios that run with evidence verification when --evidence is set.
EVIDENCE_SCENARIOS=("evidence-signing")

# --- Defaults ---
TFCLASSIFY_BIN=""
BUILD=false
EVIDENCE=false
PLAN_ONLY=false
TESTS=()

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Run e2e tests locally against Azure infrastructure.
Assumes you are already signed in with the Azure CLI.

Options:
  --build              Build tfclassify and plugin from source
  --binary PATH        Use a prebuilt tfclassify binary (plugin must be discoverable)
  --evidence           Enable evidence signing and verification
  --plan-only          Skip apply/destroy for all scenarios (faster iteration)
  -t, --test NAME      Run only the named scenario (repeatable)
  -h, --help           Show this help message

Scenarios that always run plan-only regardless of --plan-only flag:
  custom-role-cross-reference   (requires roleDefinitions/write)
  evidence-signing              (only needs plan for verification)

Examples:
  $(basename "$0") --build
  $(basename "$0") --build --plan-only
  $(basename "$0") --build -t route-table -t blast-radius
  $(basename "$0") --binary ./bin/tfclassify --evidence
EOF
  exit 0
}

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --build)     BUILD=true; shift ;;
    --binary)    TFCLASSIFY_BIN="$2"; shift 2 ;;
    --evidence)  EVIDENCE=true; shift ;;
    --plan-only) PLAN_ONLY=true; shift ;;
    -t|--test)   TESTS+=("$2"); shift 2 ;;
    -h|--help)   usage ;;
    *)           echo -e "${RED}Unknown option: $1${NC}" >&2; echo; usage ;;
  esac
done

if [[ "$BUILD" == false && -z "$TFCLASSIFY_BIN" ]]; then
  echo -e "${RED}Error: specify --build or --binary PATH${NC}" >&2
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

# Resolve binary to absolute path
if [[ ! "$TFCLASSIFY_BIN" = /* ]]; then
  TFCLASSIFY_BIN="$(pwd)/$TFCLASSIFY_BIN"
fi

if [[ ! -x "$TFCLASSIFY_BIN" ]]; then
  echo -e "${RED}Error: $TFCLASSIFY_BIN not found or not executable${NC}" >&2
  exit 1
fi

echo -e "${CYAN}Binary:${NC}    $TFCLASSIFY_BIN"
"$TFCLASSIFY_BIN" --version 2>/dev/null || true
echo -e "${CYAN}Terraform:${NC} $(terraform --version -json 2>/dev/null | jq -r .terraform_version 2>/dev/null || terraform --version | head -1)"
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
if [[ "$PLAN_ONLY" == true ]]; then
  echo -e "${YELLOW}Plan-only mode: skipping apply/destroy for all scenarios${NC}"
fi
echo ""

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
}

# --- Run a single scenario ---
run_scenario() {
  local name="$1"
  local scenario_dir="$E2E_DIR/$name"
  local log_file="$RESULTS_DIR/$name.log"
  local is_plan_only="$PLAN_ONLY"
  local use_evidence=false
  local failed=false
  local applied=false

  # Force plan-only for scenarios that require it
  if in_array "$name" "${ALWAYS_PLAN_ONLY[@]}"; then
    is_plan_only=true
  fi

  # Enable evidence for evidence scenarios when --evidence is set
  if [[ "$EVIDENCE" == true ]] && in_array "$name" "${EVIDENCE_SCENARIOS[@]}"; then
    use_evidence=true
  fi

  {
    echo "=== $name ==="
    echo ""

    # Export signing key for evidence scenarios
    if [[ "$use_evidence" == true ]]; then
      export TFCLASSIFY_SIGNING_KEY="$EVIDENCE_PRIVATE_KEY"
    fi

    # Validate config
    echo "--- Validate config ---"
    if ! "$TFCLASSIFY_BIN" validate -c "$scenario_dir/.tfclassify.hcl" 2>&1; then
      echo "FAIL: config validation"
      echo "FAIL" > "$RESULTS_DIR/$name.status"
      return
    fi

    # Terraform init
    echo "--- Terraform init ---"
    if ! terraform -chdir="$scenario_dir" init -input=false 2>&1; then
      echo "FAIL: terraform init"
      echo "FAIL" > "$RESULTS_DIR/$name.status"
      return
    fi

    # ---- Create phase ----
    echo "--- Create plan ---"
    if ! terraform -chdir="$scenario_dir" plan \
           -var-file=../e2e.tfvars -out=create.tfplan -input=false 2>&1; then
      echo "FAIL: terraform plan (create)"
      echo "FAIL" > "$RESULTS_DIR/$name.status"
      cleanup_scenario "$scenario_dir"
      return
    fi
    terraform -chdir="$scenario_dir" show -json create.tfplan \
      > "$scenario_dir/create.json" 2>&1

    local expected_create
    expected_create="$(jq -r '.create.exit_code' "$scenario_dir/expected.json")"

    # Classify create (JSON)
    local evidence_flag=""
    if [[ "$use_evidence" == true ]]; then
      evidence_flag="--evidence-file $RESULTS_DIR/$name-evidence-create-json.json"
    fi

    echo "--- Classify create (JSON) ---"
    set +e
    # shellcheck disable=SC2086
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/create.json" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o json --detailed-exitcode $evidence_flag \
      > "$scenario_dir/create-result.json" 2>&1
    local actual_create=$?
    set -e

    if [[ "$actual_create" != "$expected_create" ]]; then
      echo "FAIL: create (JSON) expected exit $expected_create, got $actual_create"
      jq . "$scenario_dir/create-result.json" 2>/dev/null || true
      failed=true
    else
      echo "PASS: create (JSON) exit code $actual_create"
    fi

    # Verify evidence (create JSON)
    if [[ "$use_evidence" == true && -f "$RESULTS_DIR/$name-evidence-create-json.json" ]]; then
      if "$TFCLASSIFY_BIN" verify \
           --evidence-file "$RESULTS_DIR/$name-evidence-create-json.json" \
           --public-key "$EVIDENCE_PUBLIC_KEY" 2>&1; then
        echo "PASS: evidence verification (create JSON)"
      else
        echo "FAIL: evidence verification (create JSON)"
        failed=true
      fi
    fi

    # Classify create (SARIF)
    echo "--- Classify create (SARIF) ---"
    set +e
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/create.json" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o sarif --detailed-exitcode > /dev/null 2>&1
    local actual_create_sarif=$?
    set -e

    if [[ "$actual_create_sarif" != "$expected_create" ]]; then
      echo "FAIL: create (SARIF) expected exit $expected_create, got $actual_create_sarif"
      failed=true
    else
      echo "PASS: create (SARIF) exit code $actual_create_sarif"
    fi

    # Classify create (binary plan)
    evidence_flag=""
    if [[ "$use_evidence" == true ]]; then
      evidence_flag="--evidence-file $RESULTS_DIR/$name-evidence-create-binary.json"
    fi

    echo "--- Classify create (binary) ---"
    set +e
    # shellcheck disable=SC2086
    "$TFCLASSIFY_BIN" \
      -p "$scenario_dir/create.tfplan" \
      -c "$scenario_dir/.tfclassify.hcl" \
      -o json --detailed-exitcode $evidence_flag \
      > "$scenario_dir/create-result-binary.json" 2>&1
    local actual_create_bin=$?
    set -e

    if [[ "$actual_create_bin" != "$expected_create" ]]; then
      echo "FAIL: create (binary) expected exit $expected_create, got $actual_create_bin"
      jq . "$scenario_dir/create-result-binary.json" 2>/dev/null || true
      failed=true
    else
      echo "PASS: create (binary) exit code $actual_create_bin"
    fi

    # Verify evidence (create binary)
    if [[ "$use_evidence" == true && -f "$RESULTS_DIR/$name-evidence-create-binary.json" ]]; then
      if "$TFCLASSIFY_BIN" verify \
           --evidence-file "$RESULTS_DIR/$name-evidence-create-binary.json" \
           --public-key "$EVIDENCE_PUBLIC_KEY" 2>&1; then
        echo "PASS: evidence verification (create binary)"
      else
        echo "FAIL: evidence verification (create binary)"
        failed=true
      fi
    fi

    # ---- Apply + Destroy phase ----
    if [[ "$is_plan_only" == true ]]; then
      echo "SKIP: apply/destroy (plan-only)"
    else
      echo "--- Apply ---"
      if ! terraform -chdir="$scenario_dir" apply -auto-approve create.tfplan 2>&1; then
        echo "FAIL: terraform apply"
        echo "--- Cleanup (forced) ---"
        terraform -chdir="$scenario_dir" destroy -auto-approve -var-file=../e2e.tfvars 2>&1 || true
        echo "FAIL" > "$RESULTS_DIR/$name.status"
        cleanup_scenario "$scenario_dir"
        return
      fi
      applied=true

      echo "--- Destroy plan ---"
      if ! terraform -chdir="$scenario_dir" plan -destroy \
             -var-file=../e2e.tfvars -out=destroy.tfplan -input=false 2>&1; then
        echo "FAIL: terraform plan (destroy)"
        terraform -chdir="$scenario_dir" destroy -auto-approve -var-file=../e2e.tfvars 2>&1 || true
        echo "FAIL" > "$RESULTS_DIR/$name.status"
        cleanup_scenario "$scenario_dir"
        return
      fi
      terraform -chdir="$scenario_dir" show -json destroy.tfplan \
        > "$scenario_dir/destroy.json" 2>&1

      local expected_destroy
      expected_destroy="$(jq -r '.destroy.exit_code' "$scenario_dir/expected.json")"

      # Classify destroy (JSON)
      evidence_flag=""
      if [[ "$use_evidence" == true ]]; then
        evidence_flag="--evidence-file $RESULTS_DIR/$name-evidence-destroy-json.json"
      fi

      echo "--- Classify destroy (JSON) ---"
      set +e
      # shellcheck disable=SC2086
      "$TFCLASSIFY_BIN" \
        -p "$scenario_dir/destroy.json" \
        -c "$scenario_dir/.tfclassify.hcl" \
        -o json --detailed-exitcode $evidence_flag \
        > "$scenario_dir/destroy-result.json" 2>&1
      local actual_destroy=$?
      set -e

      if [[ "$actual_destroy" != "$expected_destroy" ]]; then
        echo "FAIL: destroy (JSON) expected exit $expected_destroy, got $actual_destroy"
        jq . "$scenario_dir/destroy-result.json" 2>/dev/null || true
        failed=true
      else
        echo "PASS: destroy (JSON) exit code $actual_destroy"
      fi

      # Classify destroy (SARIF)
      echo "--- Classify destroy (SARIF) ---"
      set +e
      "$TFCLASSIFY_BIN" \
        -p "$scenario_dir/destroy.json" \
        -c "$scenario_dir/.tfclassify.hcl" \
        -o sarif --detailed-exitcode > /dev/null 2>&1
      local actual_destroy_sarif=$?
      set -e

      if [[ "$actual_destroy_sarif" != "$expected_destroy" ]]; then
        echo "FAIL: destroy (SARIF) expected exit $expected_destroy, got $actual_destroy_sarif"
        failed=true
      else
        echo "PASS: destroy (SARIF) exit code $actual_destroy_sarif"
      fi

      # Classify destroy (binary plan)
      evidence_flag=""
      if [[ "$use_evidence" == true ]]; then
        evidence_flag="--evidence-file $RESULTS_DIR/$name-evidence-destroy-binary.json"
      fi

      echo "--- Classify destroy (binary) ---"
      set +e
      # shellcheck disable=SC2086
      "$TFCLASSIFY_BIN" \
        -p "$scenario_dir/destroy.tfplan" \
        -c "$scenario_dir/.tfclassify.hcl" \
        -o json --detailed-exitcode $evidence_flag \
        > "$scenario_dir/destroy-result-binary.json" 2>&1
      local actual_destroy_bin=$?
      set -e

      if [[ "$actual_destroy_bin" != "$expected_destroy" ]]; then
        echo "FAIL: destroy (binary) expected exit $expected_destroy, got $actual_destroy_bin"
        jq . "$scenario_dir/destroy-result-binary.json" 2>/dev/null || true
        failed=true
      else
        echo "PASS: destroy (binary) exit code $actual_destroy_bin"
      fi

      # Verify evidence (destroy)
      if [[ "$use_evidence" == true ]]; then
        if [[ -f "$RESULTS_DIR/$name-evidence-destroy-json.json" ]]; then
          if "$TFCLASSIFY_BIN" verify \
               --evidence-file "$RESULTS_DIR/$name-evidence-destroy-json.json" \
               --public-key "$EVIDENCE_PUBLIC_KEY" 2>&1; then
            echo "PASS: evidence verification (destroy JSON)"
          else
            echo "FAIL: evidence verification (destroy JSON)"
            failed=true
          fi
        fi
        if [[ -f "$RESULTS_DIR/$name-evidence-destroy-binary.json" ]]; then
          if "$TFCLASSIFY_BIN" verify \
               --evidence-file "$RESULTS_DIR/$name-evidence-destroy-binary.json" \
               --public-key "$EVIDENCE_PUBLIC_KEY" 2>&1; then
            echo "PASS: evidence verification (destroy binary)"
          else
            echo "FAIL: evidence verification (destroy binary)"
            failed=true
          fi
        fi
      fi

      # Cleanup infrastructure
      echo "--- Cleanup ---"
      terraform -chdir="$scenario_dir" destroy -auto-approve -var-file=../e2e.tfvars 2>&1 || true
    fi

    # Clean up generated plan files
    cleanup_scenario "$scenario_dir"

    if [[ "$failed" == true ]]; then
      echo "FAIL" > "$RESULTS_DIR/$name.status"
    else
      echo "PASS" > "$RESULTS_DIR/$name.status"
    fi
  } > "$log_file" 2>&1
}

# --- Launch all scenarios in parallel ---
PIDS=()
PID_SCENARIOS=()
for scenario in "${SCENARIOS[@]}"; do
  run_scenario "$scenario" &
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
  status_file="$RESULTS_DIR/$scenario.status"
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
  status_file="$RESULTS_DIR/$scenario.status"
  if [[ -f "$status_file" && "$(cat "$status_file")" == "PASS" ]]; then
    PASSED=$((PASSED + 1))
  else
    FAILED=$((FAILED + 1))
    ERRORS+=("$scenario")
  fi
done

echo -e "${BOLD}Results: ${GREEN}$PASSED passed${NC}, ${RED}$FAILED failed${NC} (${#SCENARIOS[@]} total)"

# Print logs for failed scenarios
if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo ""
  echo -e "${BOLD}Failed scenario logs:${NC}"
  for scenario in "${ERRORS[@]}"; do
    echo ""
    echo -e "${RED}=== $scenario ===${NC}"
    cat "$RESULTS_DIR/$scenario.log"
  done
fi

# Cleanup temp files
rm -rf "$RESULTS_DIR"
if [[ -n "$EVIDENCE_PRIVATE_KEY" ]]; then
  rm -f "$EVIDENCE_PRIVATE_KEY" "$EVIDENCE_PUBLIC_KEY"
fi

if [[ $FAILED -gt 0 ]]; then
  exit 1
fi
