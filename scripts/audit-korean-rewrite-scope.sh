#!/usr/bin/env sh
set -eu

# 한국어 재작성 Epic의 범위와 제외 대상을 한곳에서 검증한다.
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

all_md="$tmp_dir/all-md.txt"
scoped_md="$tmp_dir/scoped-md.txt"
readme_md="$tmp_dir/readme-md.txt"
operating_docs="$tmp_dir/operating-docs.txt"
manual_en="$tmp_dir/manual-en.txt"
manual_ko="$tmp_dir/manual-ko.txt"
manual_delta="$tmp_dir/manual-delta.txt"

git ls-files '*.md' | sort > "$all_md"
grep -E '(^|/)README(\.ko)?\.md$' "$all_md" > "$readme_md" || true
grep -E '(^|/)(AGENTS|CLAUDE)\.md$' "$all_md" > "$operating_docs" || true

grep -Ev '(^|/)README(\.ko)?\.md$' "$all_md" \
  | grep -Ev '(^|/)(AGENTS|CLAUDE)\.md$' \
  | grep -Ev '^docs/manual/(en|ko)/' \
  > "$scoped_md" || true

if grep -E '(^|/)README(\.ko)?\.md$|(^|/)(AGENTS|CLAUDE)\.md$|^docs/manual/(en|ko)/' "$scoped_md" >/dev/null; then
  echo "error: excluded Markdown file leaked into scoped list" >&2
  exit 1
fi

sed -n 's#^docs/manual/en/##p' "$all_md" | sort > "$manual_en"
sed -n 's#^docs/manual/ko/##p' "$all_md" | sort > "$manual_ko"
if [ -s "$manual_en" ] || [ -s "$manual_ko" ]; then
  if ! diff -u "$manual_en" "$manual_ko" > "$manual_delta"; then
    echo "error: docs/manual/en and docs/manual/ko are not parity-matched" >&2
    cat "$manual_delta" >&2
    exit 1
  fi
fi

if command -v rg >/dev/null 2>&1; then
  go_comment_files=$(rg -l '//|/\*|\*/' --glob '*.go' | sort | wc -l | tr -d ' ')
else
  go_comment_files=$(git ls-files '*.go' | xargs grep -lE '//|/\*|\*/' | sort | wc -l | tr -d ' ')
fi

printf 'Korean rewrite scope audit\n'
printf 'markdown_total=%s\n' "$(wc -l < "$all_md" | tr -d ' ')"
printf 'readme_excluded=%s\n' "$(wc -l < "$readme_md" | tr -d ' ')"
printf 'operating_docs_excluded=%s\n' "$(wc -l < "$operating_docs" | tr -d ' ')"
printf 'single_language_markdown_scoped=%s\n' "$(wc -l < "$scoped_md" | tr -d ' ')"
printf 'go_files_total=%s\n' "$(git ls-files '*.go' | wc -l | tr -d ' ')"
printf 'go_comment_files_scoped=%s\n' "$go_comment_files"
printf 'manual_en_files=%s\n' "$(wc -l < "$manual_en" | tr -d ' ')"
printf 'manual_ko_files=%s\n' "$(wc -l < "$manual_ko" | tr -d ' ')"
printf '\nScoped Markdown groups:\n'
awk -F/ '
  NF == 1 { print "  " $1; next }
  $1 == "docs" && $2 == "superpowers" { print "  " $1 "/" $2 "/" $3; next }
  { print "  " $1 "/" $2 }
' "$scoped_md" | sort | uniq -c
