# 한국어 재작성 범위 감사

## 배경

이 저장소의 문서와 예제 주석을 한국어로 정리하려면, 먼저 어떤 파일이 재작성 대상이고 어떤 파일이 제외 대상인지 고정해야 한다. README 쌍은 이미 별도 다국어 문서로 관리되고, `AGENTS.md` 같은 LLM-facing 운영 문서는 영어 유지 대상이다. 따라서 이 작업은 재작성 자체보다 범위 누락과 제외 대상 오염을 막는 guard를 먼저 둔다.

## 결정

- `scripts/audit-korean-rewrite-scope.sh`를 범위 감사의 기준 명령으로 둔다.
- `README.md`, `README.ko.md`, `AGENTS.md`, `CLAUDE.md`, `docs/manual/en/**`, `docs/manual/ko/**`는 한국어 재작성 primary scope에서 제외한다.
- 단일 언어 Markdown은 `CHANGELOG.md`, `WIP.md`, `docs/lessons/**`, `docs/review/**`, `docs/superpowers/{plans,research,reviews,specs}/**`로 계산한다.
- Go 파일은 실행 동작이 아니라 package, type, function, method, field, argument 의미를 설명하는 주석을 한국어화 대상으로 본다.

## 검증

`make docs-audit`는 scoped Markdown 수, README 제외 수, 운영 문서 제외 수, Go comment 후보 파일 수, manual EN/KO parity를 출력한다. 이후 PR train의 마지막 감사 단계는 이 명령을 다시 실행해서 README와 운영 문서가 재작성 범위에 섞이지 않았음을 확인해야 한다.
