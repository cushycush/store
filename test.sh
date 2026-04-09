#!/usr/bin/env bash
set -uo pipefail

# Acceptance test script for store CLI.
# Exercises every command and feature against a real temporary environment.
#
# Usage:
#   ./test.sh              # build and test
#   ./test.sh ./store      # test an existing binary

PASS=0
FAIL=0

red()   { printf '\033[1;31m%s\033[0m' "$*"; }
green() { printf '\033[1;32m%s\033[0m' "$*"; }
bold()  { printf '\033[1m%s\033[0m' "$*"; }

pass() { PASS=$((PASS + 1)); printf '  %s %s\n' "$(green '✓')" "$1"; }
fail() { FAIL=$((FAIL + 1)); printf '  %s %s — %s\n' "$(red '✗')" "$1" "$2"; }

is_symlink()    { [[ -L "$1" ]]; }
not_exists()    { [[ ! -e "$1" ]] && [[ ! -L "$1" ]]; }
S() { "$STORE_BIN" "$@"; }

STORE_BIN="${1:-}"
if [[ -z "$STORE_BIN" ]]; then
    printf '%s\n' "$(bold 'Building store...')"
    go build -o ./store ./cmd/store
    STORE_BIN="./store"
fi
STORE_BIN="$(realpath "$STORE_BIN")"

TMPDIR_ROOT=$(mktemp -d)
trap 'rm -rf "$TMPDIR_ROOT"' EXIT

export HOME="$TMPDIR_ROOT/home"
export XDG_STATE_HOME="$TMPDIR_ROOT/state"
export STORE_PASSPHRASE="testpass123"
mkdir -p "$HOME" "$XDG_STATE_HOME"

REPO="$TMPDIR_ROOT/dotfiles"
mkdir -p "$REPO"
cd "$REPO"

# ============================================================
printf '\n%s\n' "$(bold '=== store init ===')"
# ============================================================

S init >/dev/null 2>&1
if [[ -f .store/config.yaml ]]; then pass "init creates .store/config.yaml"; else fail "init creates .store/config.yaml" "missing config"; fi

if S init >/dev/null 2>&1; then fail "init fails if already initialized" "should have failed"; else pass "init fails if already initialized"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== store version ===')"
# ============================================================

out=$(S version 2>&1)
if [[ "$out" == *"store version"* ]]; then pass "version prints version string"; else fail "version prints version string" "got: $out"; fi

out=$(S --version 2>&1)
if [[ "$out" == *"store version"* ]]; then pass "--version flag works"; else fail "--version flag works" "got: $out"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== store add (whole directory) ===')"
# ============================================================

TARGET_NVIM="$TMPDIR_ROOT/targets/nvim"
mkdir -p "$REPO/nvim/lua"
echo "vim.opt.number = true" > "$REPO/nvim/init.lua"
echo "return {}" > "$REPO/nvim/lua/plugins.lua"

S add nvim -t "$TARGET_NVIM" >/dev/null 2>&1
if is_symlink "$TARGET_NVIM"; then pass "add creates whole-directory symlink"; else fail "add creates whole-directory symlink" "not a symlink"; fi

target=$(readlink "$TARGET_NVIM")
if [[ "$target" == *"/nvim" ]]; then pass "symlink points to repo directory"; else fail "symlink points to repo directory" "points to: $target"; fi

if [[ -f "$TARGET_NVIM/init.lua" ]] && [[ -f "$TARGET_NVIM/lua/plugins.lua" ]]; then
    pass "files accessible through symlink"
else
    fail "files accessible through symlink" "files not accessible"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== store add (file mode — explicit files) ===')"
# ============================================================

TARGET_SHELLS="$TMPDIR_ROOT/targets/shells"
mkdir -p "$TARGET_SHELLS" "$REPO/shells"
echo 'export ZSH=true' > "$REPO/shells/.zshrc"
echo 'export BASH=true' > "$REPO/shells/.bashrc"
echo 'set fish' > "$REPO/shells/config.fish"

S add shells -t "$TARGET_SHELLS" -f .zshrc -f .bashrc >/dev/null 2>&1
if is_symlink "$TARGET_SHELLS/.zshrc" && is_symlink "$TARGET_SHELLS/.bashrc"; then
    pass "add with -f creates per-file symlinks"
else
    fail "add with -f creates per-file symlinks" "missing symlinks"
fi

if not_exists "$TARGET_SHELLS/config.fish"; then pass "non-listed file is not symlinked"; else fail "non-listed file is not symlinked" "config.fish exists"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== store add (file mode — glob patterns) ===')"
# ============================================================

TARGET_CONFIGS="$TMPDIR_ROOT/targets/configs"
mkdir -p "$REPO/configs/sub"
echo "a" > "$REPO/configs/app.conf"
echo "b" > "$REPO/configs/sub/db.conf"
echo "c" > "$REPO/configs/readme.txt"

S add configs -t "$TARGET_CONFIGS" -p "**/*.conf" >/dev/null 2>&1
if is_symlink "$TARGET_CONFIGS/app.conf" && is_symlink "$TARGET_CONFIGS/sub/db.conf"; then
    pass "add with -p creates pattern-matched symlinks"
else
    fail "add with -p creates pattern-matched symlinks" "missing symlinks"
fi

if not_exists "$TARGET_CONFIGS/readme.txt"; then pass "non-matching file is not symlinked"; else fail "non-matching file is not symlinked" "readme.txt exists"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== store status ===')"
# ============================================================

out=$(S status 2>&1)
if [[ "$out" == *"nvim"* ]] && [[ "$out" == *"shells"* ]] && [[ "$out" == *"configs"* ]]; then
    pass "status shows all stores"
else
    fail "status shows all stores" "missing stores"
fi

out=$(S status nvim 2>&1)
if [[ "$out" == *"linked"* ]]; then pass "status shows linked indicator"; else fail "status shows linked indicator" "got: $out"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== store diff ===')"
# ============================================================

out=$(S diff 2>&1)
if [[ "$out" == *"ok"* ]]; then pass "diff shows ok for linked stores"; else fail "diff shows ok for linked stores" "got: $out"; fi

out=$(S diff --only nvim 2>&1)
if [[ "$out" == *"nvim"* ]] && [[ "$out" != *"shells"* ]]; then
    pass "diff --only filters stores"
else
    fail "diff --only filters stores" "filter not working"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== store modify ===')"
# ============================================================

TARGET_SHELLS2="$TMPDIR_ROOT/targets/shells2"
mkdir -p "$TARGET_SHELLS2"

S modify shells -t "$TARGET_SHELLS2" -f .zshrc >/dev/null 2>&1
if is_symlink "$TARGET_SHELLS2/.zshrc"; then pass "modify changes target and relinks"; else fail "modify changes target and relinks" "new target not linked"; fi

if not_exists "$TARGET_SHELLS/.zshrc"; then pass "old target symlinks removed after modify"; else fail "old target symlinks removed after modify" "old symlink still exists"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== store target add / remove / modify ===')"
# ============================================================

TARGET_FISH="$TMPDIR_ROOT/targets/fish"
TARGET_NU="$TMPDIR_ROOT/targets/nu"
mkdir -p "$TARGET_FISH" "$TARGET_NU"
echo 'set nu' > "$REPO/shells/config.nu"

S target add shells -t "$TARGET_FISH" -f config.fish >/dev/null 2>&1
if is_symlink "$TARGET_FISH/config.fish"; then pass "target add creates multi-target store"; else fail "target add creates multi-target store" "not linked"; fi

S target add shells -t "$TARGET_NU" -f config.nu >/dev/null 2>&1
if is_symlink "$TARGET_NU/config.nu"; then pass "target add second target"; else fail "target add second target" "not linked"; fi

S target modify shells -t "$TARGET_FISH" -f config.fish >/dev/null 2>&1
if is_symlink "$TARGET_FISH/config.fish"; then pass "target modify updates files"; else fail "target modify updates files" "failed"; fi

S target remove shells -t "$TARGET_NU" >/dev/null 2>&1
if not_exists "$TARGET_NU/config.nu"; then pass "target remove unlinks and removes target"; else fail "target remove unlinks and removes target" "symlink still exists"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== store remove / removeall ===')"
# ============================================================

S remove configs >/dev/null 2>&1
if not_exists "$TARGET_CONFIGS/app.conf"; then pass "remove deletes symlinks and config entry"; else fail "remove deletes symlinks and config entry" "symlink still exists"; fi

S removeall >/dev/null 2>&1
if not_exists "$TARGET_NVIM" && not_exists "$TARGET_SHELLS2/.zshrc"; then
    pass "removeall removes all remaining stores"
else
    fail "removeall removes all remaining stores" "some symlinks remain"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== store (restore all) ===')"
# ============================================================

TARGET_GIT="$TMPDIR_ROOT/targets/git"
mkdir -p "$REPO/git"
echo "[user]" > "$REPO/git/.gitconfig"

S add nvim -t "$TARGET_NVIM" >/dev/null 2>&1
S add git -t "$TARGET_GIT" >/dev/null 2>&1

rm -f "$TARGET_NVIM" "$TARGET_GIT"

S >/dev/null 2>&1
if is_symlink "$TARGET_NVIM" && is_symlink "$TARGET_GIT"; then
    pass "store restores all symlinks"
else
    fail "store restores all symlinks" "not all restored"
fi

rm -f "$TARGET_NVIM" "$TARGET_GIT"
S --only nvim >/dev/null 2>&1
if is_symlink "$TARGET_NVIM" && ! is_symlink "$TARGET_GIT" 2>/dev/null; then
    pass "store --only restores only named stores"
else
    fail "store --only restores only named stores" "filter not applied"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== secrets ===')"
# ============================================================

S secret set github_token ghp_testtoken123 >/dev/null 2>&1
pass "secret set stores a secret"

out=$(S secret get github_token 2>&1)
if [[ "$out" == *"ghp_testtoken123"* ]]; then pass "secret get retrieves the secret"; else fail "secret get retrieves the secret" "got: $out"; fi

out=$(S secret list 2>&1)
if [[ "$out" == *"github_token"* ]]; then pass "secret list shows secret names"; else fail "secret list shows secret names" "got: $out"; fi

S secret set api_key sk-testapikey >/dev/null 2>&1
out=$(S secret list 2>&1)
if [[ "$out" == *"api_key"* ]] && [[ "$out" == *"github_token"* ]]; then
    pass "secret set second secret"
else
    fail "secret set second secret" "got: $out"
fi

S secret rm api_key >/dev/null 2>&1
out=$(S secret list 2>&1)
if [[ "$out" != *"api_key"* ]]; then pass "secret rm removes a secret"; else fail "secret rm removes a secret" "api_key still present"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== secret template rendering ===')"
# ============================================================

TARGET_TMPL="$TMPDIR_ROOT/targets/tmpl"
mkdir -p "$REPO/tmpl"
printf 'token = {{ secret "github_token" }}\n' > "$REPO/tmpl/config.ini"
echo 'plain = hello' > "$REPO/tmpl/plain.txt"

S add tmpl -t "$TARGET_TMPL" >/dev/null 2>&1
if [[ -f "$TARGET_TMPL/config.ini" ]]; then
    content=$(cat "$TARGET_TMPL/config.ini")
    if [[ "$content" == *"ghp_testtoken123"* ]] && [[ "$content" != *"{{ secret"* ]]; then
        pass "store renders templates with secret values"
    else
        fail "store renders templates with secret values" "template not rendered: $content"
    fi
else
    fail "store renders templates with secret values" "config.ini not found"
fi

if [[ -f "$TARGET_TMPL/plain.txt" ]]; then pass "non-template file still accessible"; else fail "non-template file still accessible" "plain.txt missing"; fi

S remove tmpl >/dev/null 2>&1

# ============================================================
printf '\n%s\n' "$(bold '=== platform conditionals (when clause) ===')"
# ============================================================

TARGET_PLAT="$TMPDIR_ROOT/targets/plat"
mkdir -p "$REPO/plattest"
echo "test" > "$REPO/plattest/file.txt"

printf '    plattest:\n        target: %s\n        when:\n            os: impossibleos\n' "$TARGET_PLAT" >> .store/config.yaml

S >/dev/null 2>&1
if not_exists "$TARGET_PLAT"; then pass "store with non-matching when clause is skipped"; else fail "store with non-matching when clause is skipped" "should have been skipped"; fi

out=$(S status 2>&1)
if [[ "$out" != *"plattest"* ]]; then pass "status skips non-matching platform stores"; else fail "status skips non-matching platform stores" "plattest shown"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== ignore patterns ===')"
# ============================================================

TARGET_IGN="$TMPDIR_ROOT/targets/ign"
mkdir -p "$REPO/ignored"
echo "keep" > "$REPO/ignored/keep.txt"
echo "skip" > "$REPO/ignored/skip.bak"
mkdir -p "$REPO/ignored/.git"
echo "x" > "$REPO/ignored/.git/config"

S add ignored -t "$TARGET_IGN" >/dev/null 2>&1
if [[ -f "$TARGET_IGN/keep.txt" ]] && [[ ! -e "$TARGET_IGN/.git" ]]; then
    pass "global ignores exclude .git from whole-dir stores"
else
    fail "global ignores exclude .git from whole-dir stores" ".git was not excluded or keep.txt missing"
fi

S remove ignored >/dev/null 2>&1
mkdir -p "$REPO/ign2"
echo "a" > "$REPO/ign2/good.txt"
echo "b" > "$REPO/ign2/bad.bak"
S add ign2 -t "${TARGET_IGN}2" >/dev/null 2>&1
S remove ign2 >/dev/null 2>&1
rm -rf "${TARGET_IGN}2"
cat > .store/config.yaml <<YAMLEOF
stores:
    ign2:
        target: ${TARGET_IGN}2
        ignore:
            - "*.bak"
YAMLEOF
S >/dev/null 2>&1
if [[ -f "${TARGET_IGN}2/good.txt" ]] && [[ ! -e "${TARGET_IGN}2/bad.bak" ]]; then
    pass "explicit ignore pattern excludes matching files"
else
    fail "explicit ignore pattern excludes matching files" "ignore pattern not applied"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== hooks ===')"
# ============================================================

TARGET_HOOKS="$TMPDIR_ROOT/targets/hooks"
HOOK_LOG="$TMPDIR_ROOT/hook.log"
mkdir -p "$REPO/hooked"
echo "data" > "$REPO/hooked/file.txt"

printf '    hooked:\n        target: %s\n        hooks:\n            post: "echo hook_ran > %s"\n' "$TARGET_HOOKS" "$HOOK_LOG" >> .store/config.yaml

S --only hooked >/dev/null 2>&1
if [[ -f "$HOOK_LOG" ]] && grep -q "hook_ran" "$HOOK_LOG"; then
    pass "per-store post hook runs after linking"
else
    fail "per-store post hook runs after linking" "hook did not run"
fi

GLOBAL_LOG="$TMPDIR_ROOT/global_hook.log"
mkdir -p .store/hooks
cat > .store/hooks/pre-store <<HOOKEOF
#!/bin/sh
echo "global_pre" > $GLOBAL_LOG
HOOKEOF
chmod +x .store/hooks/pre-store

rm -f "$TARGET_HOOKS"
S --only hooked >/dev/null 2>&1
if [[ -f "$GLOBAL_LOG" ]] && grep -q "global_pre" "$GLOBAL_LOG"; then
    pass "global pre-store hook runs"
else
    fail "global pre-store hook runs" "global hook did not run"
fi
rm -f .store/hooks/pre-store

# ============================================================
printf '\n%s\n' "$(bold '=== conflict resolution ===')"
# ============================================================

TARGET_CONFLICT="$TMPDIR_ROOT/targets/conflict"
mkdir -p "$REPO/conflicted" "$TARGET_CONFLICT"
echo "repo version" > "$REPO/conflicted/file.txt"
echo "existing" > "$TARGET_CONFLICT/file.txt"

echo "y" | S add conflicted -t "$TARGET_CONFLICT" -f file.txt --force >/dev/null 2>&1
if is_symlink "$TARGET_CONFLICT/file.txt"; then
    pass "store resolves conflicts when confirmed"
else
    fail "store resolves conflicts when confirmed" "conflict not resolved"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== store doctor ===')"
# ============================================================

if S doctor >/dev/null 2>&1; then pass "doctor runs without error on healthy config"; else fail "doctor runs without error on healthy config" "doctor failed"; fi

printf '    ghost:\n        target: /tmp/ghost\n' >> .store/config.yaml

out=$(S doctor 2>&1) || true
if [[ "$out" == *"ghost"* ]] && [[ "$out" == *"no directory"* ]]; then
    pass "doctor detects orphaned config entry"
else
    fail "doctor detects orphaned config entry" "orphan not detected"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== store import ===')"
# ============================================================

IMPORT_REPO="$TMPDIR_ROOT/import-repo"
IMPORT_SCAN="$TMPDIR_ROOT/import-scan"
mkdir -p "$IMPORT_REPO/myapp" "$IMPORT_SCAN"
echo "app config" > "$IMPORT_REPO/myapp/config.yaml"
pushd "$IMPORT_REPO" >/dev/null
"$STORE_BIN" init >/dev/null 2>&1
ln -s "$IMPORT_REPO/myapp" "$IMPORT_SCAN/myapp"

out=$("$STORE_BIN" import --scan-dir "$IMPORT_SCAN" --dry-run 2>&1)
if [[ "$out" == *"myapp"* ]]; then pass "import --dry-run discovers symlinks"; else fail "import --dry-run discovers symlinks" "got: $out"; fi

"$STORE_BIN" import --scan-dir "$IMPORT_SCAN" --force >/dev/null 2>&1
if grep -q "myapp" .store/config.yaml; then pass "import with --force writes config"; else fail "import with --force writes config" "not in config"; fi

popd >/dev/null

# ============================================================
printf '\n%s\n' "$(bold '=== store completion ===')"
# ============================================================

for shell in bash zsh fish powershell; do
    out=$(S completion "$shell" 2>&1)
    if [[ -n "$out" ]]; then pass "completion $shell generates output"; else fail "completion $shell generates output" "empty"; fi
done

if S completion invalid >/dev/null 2>&1; then fail "completion rejects invalid shell" "should have failed"; else pass "completion rejects invalid shell"; fi

# ============================================================
printf '\n%s\n' "$(bold '=== hook environment variables ===')"
# ============================================================

ENV_LOG="$TMPDIR_ROOT/env.log"
mkdir -p "$REPO/envtest"
echo "x" > "$REPO/envtest/file.txt"
TARGET_ENV="$TMPDIR_ROOT/targets/envtest"

cd "$REPO"
printf '    envtest:\n        target: %s\n        hooks:\n            post: "env > %s"\n' "$TARGET_ENV" "$ENV_LOG" >> .store/config.yaml

S --only envtest >/dev/null 2>&1
if grep -q "STORE_OS=" "$ENV_LOG" \
   && grep -q "STORE_ARCH=" "$ENV_LOG" \
   && grep -q "STORE_HOSTNAME=" "$ENV_LOG" \
   && grep -q "STORE_ROOT=" "$ENV_LOG" \
   && grep -q "STORE_ACTION=" "$ENV_LOG" \
   && grep -q "STORE_NAME=" "$ENV_LOG"; then
    pass "hooks receive platform environment variables"
else
    fail "hooks receive platform environment variables" "missing env vars"
fi

# ============================================================
printf '\n%s\n' "$(bold '=== Results ===')"
# ============================================================

printf '  %s passed, %s failed\n\n' "$(green "$PASS")" "$(red "$FAIL")"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
