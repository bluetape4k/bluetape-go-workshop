# Gin SQL order service diagram checklist 수리

## 결정

Issue #65 README diagram은 decorative asset이 아니라 source-backed teaching material로
취급해야 한다. architecture image는 static ownership map이고, create-order 및 rollback
image는 participant, lifeline, horizontal message lane, branch region, rendered PNG inspection을
갖춘 sequence-diagram family를 사용한다.

## 근거

첫 버전은 올바른 module boundary를 설명했지만 diagram pass가 현재 `bluetape4k-diagram`
checklist를 눈에 보이게 증명하지 못했다.

- sequence diagram은 rendered PNG에서 local best-practices parity가 필요하다.
- rollback behavior는 ordered behavior이므로 generic flowchart가 아니라 sequence diagram이어야
  한다.
- marker, icon, connector, label, full-size PNG, contact-sheet check를 review evidence로
  기록해야 한다.

## 규칙

이 repository의 향후 README diagram correction에서는 다음을 지킨다.

- 그리기 전에 target README와 source implementation을 읽는다.
- sequence-named asset을 가까운 sequence family example과 비교한다.
- review 전에 변경한 모든 SVG를 CairoSVG로 render한다.
- 변경한 모든 PNG를 full size로 inspect하고, 여러 diagram이 바뀌면 contact sheet도 inspect한다.
- PR 생성 전에 marker/icon/geometry audit evidence를 `docs/review/`에 기록한다.
