# 다국어 Example README 정리

## 맥락

workshop 예제에는 root-level English/Korean README가 있었지만, 개별 example directory는 대부분 English-only 문서였다. 사용자는 example README 전반의 English/Korean 지원과 scenario를 설명하는 데 필요한 diagram을 요청했다.

## 결정

모든 example `README.md` 옆에 `README.ko.md`를 추가하고, 주변 prose는 locale별로 작성하되 English-label diagram asset은 두 locale이 공유한다. 언어별 diagram을 하나씩 만들기보다, 각 예제는 `docs/images/readme-diagrams/` 아래의 작은 shared scenario diagram set을 가리킨다.

## 결과

- root README 파일에 workshop example map과 locale link가 추가됐다.
- 모든 example directory는 `README.md`와 `README.ko.md`를 모두 가진다.
- data/codec/cleanup, leadership, integration, concurrency/resilience scenario에는 PNG/SVG README diagram asset이 있다.
- node-and-connector diagram에는 DOT, Plain, Graphviz render evidence가 포함된다.

## 검증

- 새 diagram PNG를 DOT에서 모두 render했다.
- 새 final PNG를 readable size로 각각 검사했다.
- README embed가 SVG가 아니라 PNG를 사용하는지 확인했다.
- 새 diagram SVG가 site UI font stack을 사용하지 않는지 확인했다.

## 이후 Guard

새 workshop 예제를 추가할 때는 같은 변경에서 `README.md`와 `README.ko.md`를 함께 만든다. README에 diagram이 필요하면 양쪽 locale에 PNG를 embed하고, matching SVG와 structural evidence를 옆에 둔다.
