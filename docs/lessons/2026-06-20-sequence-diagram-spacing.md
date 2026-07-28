# Sequence Diagram 간격 조정

## 맥락

여러 README sequence diagram에서 call label이 다음 call line이나 branch-region label에
가까워 렌더링된 PNG가 답답하게 보였다.

## 결정

세로 row gap과 SVG canvas height를 함께 늘려 sequence 간격을 고친다. call-to-call
또는 alt-label 충돌의 주 해결책으로 label을 좌우로 옮기지 않는다. 그렇게 하면 증상만
숨기고 message lane의 답답한 간격은 그대로 남는다.

## 이후 지침

- call label이 다음 call line과 겹치면 뒤쪽 message row를 아래로 내린다.
- `height`, `viewBox`, canvas height, lifeline length, activation height,
  alt frame height, footer position을 같은 유효 delta만큼 늘린다.
- source 또는 target participant 순서가 잘못된 경우가 아니라면 participant 위치와
  label `x` 좌표는 안정적으로 유지한다.
- sequence diagram 변경을 commit하기 전에 CairoSVG로 PNG를 다시 렌더링하고,
  label-label 및 label-line overlap에 대해 id-aware sweep을 실행한다.
