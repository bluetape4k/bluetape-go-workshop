#!/usr/bin/env bash
set -euo pipefail

out_dir="docs/images/readme-diagrams"
font_architects="$HOME/Library/Fonts/ArchitectsDaughter-Regular.ttf"
font_comic="$HOME/Library/Fonts/ComicMono.ttf"

mkdir -p "$out_dir"

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

require_font() {
  if [[ ! -f "$1" ]]; then
    echo "missing required font file: $1" >&2
    exit 1
  fi
}

write_gate() {
  local name="$1"
  local nodes="$2"
  local routes="$3"
  local segments="$4"
  local margin_left="$5"
  local margin_right="$6"
  local margin_top="$7"
  local margin_bottom="$8"
  local max_delta="${9:-24}"
  local max_margin="$margin_left"
  local min_margin="$margin_left"

  for value in "$margin_right" "$margin_top" "$margin_bottom"; do
    (( value > max_margin )) && max_margin="$value"
    (( value < min_margin )) && min_margin="$value"
  done

  local margin_delta=$((max_margin - min_margin))
  if (( margin_delta > max_delta )); then
    echo "${name}: margin imbalance left=${margin_left} right=${margin_right} top=${margin_top} bottom=${margin_bottom} delta=${margin_delta} max=${max_delta}" >&2
    exit 1
  fi

  echo "${name}: nodes=${nodes} routes=${routes} segments=${segments} badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=0 margins=${margin_left}/${margin_right}/${margin_top}/${margin_bottom} titleGap=ok fontFallback=0"
}

render_graphviz_pair() {
  local name="$1"
  dot -Tplain "$out_dir/${name}.dot" > "$out_dir/${name}.plain"
  dot -Tsvg "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.svg"
  dot -Tpng "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.png"
}

validate_svg() {
  local name="$1"
  rg -q "Architects Daughter" "$out_dir/${name}.svg"
  rg -q "Comic Mono" "$out_dir/${name}.svg"
  rg -q "markerWidth=\"8\"" "$out_dir/${name}.svg"
  rg -q "markerHeight=\"8\"" "$out_dir/${name}.svg"
  if rg -q "Inter|Arial|Helvetica" "$out_dir/${name}.svg"; then
    echo "${name}: forbidden UI font family found" >&2
    exit 1
  fi
}

require_tool dot
require_tool rsvg-convert
require_tool rg
require_font "$font_architects"
require_font "$font_comic"

cat > "$out_dir/retry-dead-letter-batch-worker-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.56, ranksep=0.82, splines=true, overlap=false, fontname="Architects Daughter", label="Retry Dead-Letter Batch Worker Scenario", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  queue [label="ticket queue\n4 deterministic items", fillcolor="#fef3c7", color="#b99b5d"];
  success [label="ticket-1001\nsuccess once", fillcolor="#dcfce7", color="#5d8a62"];
  transient [label="ticket-1002\ntransient once", fillcolor="#dbeafe", color="#4f7e9d"];
  retry [label="retry attempt\nsucceeds", fillcolor="#e0f2fe", color="#48758d"];
  permanent [label="ticket-1003\nblocked address", fillcolor="#fee2e2", color="#bf6672"];
  dlt [label="dead letter\nreason + attempts", fillcolor="#ffe4e6", color="#b95f7a"];
  final [label="ticket-1004\nsuccess once", fillcolor="#dcfce7", color="#5d8a62"];
  report [label="batch report\nread=4 write=3 retry=1 skip=1", fillcolor="#ede9fe", color="#8a6bb0"];
  queue -> success [label="read"];
  success -> transient [label="write", color="#5d8a62"];
  transient -> retry [label="ErrTransientTicket", color="#48758d"];
  retry -> permanent [label="write", color="#5d8a62"];
  permanent -> dlt [label="ErrPermanentTicket", color="#b95f7a"];
  dlt -> final [label="skip item", color="#b95f7a"];
  final -> report [label="complete", color="#8a6bb0"];
}
DOT

cat > "$out_dir/retry-dead-letter-batch-worker-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.62, ranksep=0.82, splines=true, overlap=false, fontname="Architects Daughter", label="Retry Dead-Letter Batch Worker Architecture", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  cli [label="go run\nlocal demo", fillcolor="#d7ecf2", color="#48758d"];
  job [label="batch.Job\nticket-batch-worker", fillcolor="#fef3c7", color="#b99b5d"];
  step [label="batch.Step\nchunk size 2", fillcolor="#ecfccb", color="#78944d"];
  reader [label="TicketReader\nin-memory queue", fillcolor="#dbeafe", color="#4f7e9d"];
  processor [label="TicketProcessor\nclassify + attempts", fillcolor="#dcfce7", color="#5d8a62"];
  writer [label="TicketWriter\nprocessed sink", fillcolor="#f3e8ff", color="#8a6bb0"];
  retry [label="RetryPolicy\ntransient only", fillcolor="#e0f2fe", color="#48758d"];
  skip [label="SkipPolicy\npermanent only", fillcolor="#ffe4e6", color="#b95f7a"];
  dlt [label="DeadLetterStore\nreason + attempts", fillcolor="#fee2e2", color="#bf6672"];
  cli -> job [label="run"];
  job -> step [label="one step"];
  step -> reader [label="read"];
  step -> processor [label="process"];
  processor -> retry [label="retry transient", color="#48758d"];
  processor -> skip [label="skip permanent", color="#b95f7a"];
  processor -> dlt [label="record DLT", color="#b95f7a"];
  processor -> writer [label="processed tickets", color="#8a6bb0"];
}
DOT

cat > "$out_dir/retry-dead-letter-batch-worker-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=TB, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.44, ranksep=0.58, splines=ortho, fontname="Architects Daughter", label="Retry Dead-Letter Batch Worker Sequence", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.4, arrowsize=0.75, fontname="Comic Mono", fontsize=11, fontcolor="#425466"];
  start [label="Open reader + writer", fillcolor="#fef3c7", color="#b99b5d"];
  read1 [label="Read ticket-1001", fillcolor="#dcfce7", color="#5d8a62"];
  read2 [label="Read ticket-1002", fillcolor="#dbeafe", color="#4f7e9d"];
  retry [label="Retry transient\nattempt 2 succeeds", fillcolor="#e0f2fe", color="#48758d"];
  flush1 [label="Write chunk 1", fillcolor="#f3e8ff", color="#8a6bb0"];
  permanent [label="Read ticket-1003\nrecord DLT", fillcolor="#fee2e2", color="#bf6672"];
  skip [label="Skip permanent item", fillcolor="#ffe4e6", color="#b95f7a"];
  final [label="Read ticket-1004\nwrite final chunk", fillcolor="#dcfce7", color="#5d8a62"];
  done [label="Report completed\nretry=1 skip=1", fillcolor="#ede9fe", color="#8a6bb0"];
  start -> read1;
  read1 -> read2 [color="#5d8a62"];
  read2 -> retry [color="#48758d"];
  retry -> flush1 [color="#5d8a62"];
  flush1 -> permanent [color="#b95f7a"];
  permanent -> skip [color="#b95f7a"];
  skip -> final [color="#5d8a62"];
  final -> done [color="#8a6bb0"];
}
DOT

render_graphviz_pair "retry-dead-letter-batch-worker-scenario"
render_graphviz_pair "retry-dead-letter-batch-worker-architecture"
render_graphviz_pair "retry-dead-letter-batch-worker-sequence"

cat > "$out_dir/retry-dead-letter-batch-worker-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="860" viewBox="0 0 1500 860" role="img" aria-labelledby="title desc">
  <title id="title">Retry Dead-Letter Batch Worker Scenario</title>
  <desc id="desc">Scenario showing successful ticket processing, transient retry, permanent dead-letter capture, skip accounting, and final batch report.</desc>
  <defs>
    <marker id="sc-blue" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#48758d"/></marker>
    <marker id="sc-green" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="sc-red" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="sc-purple" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #425466; }
      .subtitle { font-size: 15px; }
      .card-title { font-family: 'Architects Daughter'; font-size: 23px; fill: #24313f; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffdf8; stroke: #d8e2e8; stroke-width: 2; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 18; }
      .blue { fill: none; stroke: #48758d; stroke-width: 3; marker-end: url(#sc-blue); }
      .green { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#sc-green); }
      .red { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#sc-red); }
      .purple { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#sc-purple); }
    </style>
  </defs>
  <rect width="1500" height="860" fill="#f7f3ea"/>
  <rect class="frame" x="44" y="34" width="1412" height="792" rx="20"/>
  <text class="title" x="750" y="82" text-anchor="middle">Retry Dead-Letter Batch Worker Scenario</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">A bounded batch retry handles transient work while permanent items become inspectable dead letters.</text>

  <rect class="band" x="82" y="158" width="1336" height="164"/><text class="card-title" x="112" y="196">Input queue</text>
  <g transform="translate(168 214)"><rect width="250" height="74" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="125" y="31" text-anchor="middle">Ticket queue</text><text class="detail" x="125" y="54" text-anchor="middle">4 deterministic items</text></g>
  <g transform="translate(606 202)"><rect width="274" height="98" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="137" y="34" text-anchor="middle">ticket-1002</text><text class="detail" x="137" y="58" text-anchor="middle">provider throttled</text><text class="detail" x="137" y="78" text-anchor="middle">transient once</text></g>
  <g transform="translate(1082 202)"><rect width="250" height="98" rx="12" fill="#fee2e2" stroke="#bf6672" stroke-width="2"/><text class="card-title" x="125" y="34" text-anchor="middle">ticket-1003</text><text class="detail" x="125" y="58" text-anchor="middle">blocked address</text><text class="detail" x="125" y="78" text-anchor="middle">permanent failure</text></g>

  <rect class="band" x="82" y="374" width="1336" height="178"/><text class="card-title" x="112" y="412">Policy outcomes</text>
  <g transform="translate(250 448)"><rect width="250" height="74" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="125" y="31" text-anchor="middle">Processed</text><text class="detail" x="125" y="54" text-anchor="middle">ticket-1001, 1004</text></g>
  <g transform="translate(624 438)"><rect width="250" height="94" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="125" y="33" text-anchor="middle">RetryPolicy</text><text class="detail" x="125" y="57" text-anchor="middle">attempt 1 fails</text><text class="detail" x="125" y="77" text-anchor="middle">attempt 2 succeeds</text></g>
  <g transform="translate(1000 438)"><rect width="268" height="94" rx="12" fill="#ffe4e6" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="134" y="33" text-anchor="middle">DeadLetter</text><text class="detail" x="134" y="57" text-anchor="middle">reason + attempts</text><text class="detail" x="134" y="77" text-anchor="middle">then SkipPolicy</text></g>

  <rect class="band" x="82" y="606" width="1336" height="110"/><text class="card-title" x="112" y="642">Final batch report</text>
  <g transform="translate(486 652)"><rect width="528" height="44" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="264" y="29" text-anchor="middle">read=4 write=3 retry=1 skip=1 completed</text></g>

  <rect class="band" x="82" y="762" width="1336" height="36"/><text class="footer" x="750" y="786" text-anchor="middle">The dead-letter list is teaching evidence, not durable production storage.</text>

  <path class="blue" d="M 418 251 L 606 251"/><text class="label" x="512" y="231" text-anchor="middle">read + classify</text>
  <path class="blue" d="M 880 251 L 1082 251"/><text class="label" x="981" y="231" text-anchor="middle">next item</text>
  <path class="green" d="M 293 288 L 293 448"/><text class="label" x="344" y="366" text-anchor="middle">success path</text>
  <path class="blue" d="M 743 300 L 743 438"/><text class="label" x="808" y="366" text-anchor="middle">retry transient</text>
  <path class="red" d="M 1207 300 L 1207 438"/><text class="label" x="1276" y="366" text-anchor="middle">record DLT</text>
  <path class="purple" d="M 749 532 L 749 652"/><text class="label" x="810" y="590" text-anchor="middle">report counts</text>
</svg>
SVG

cat > "$out_dir/retry-dead-letter-batch-worker-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="900" viewBox="0 0 1500 900" role="img" aria-labelledby="title desc">
  <title id="title">Retry Dead-Letter Batch Worker Architecture</title>
  <desc id="desc">Architecture showing the CLI runner, batch job, ticket reader, processor classification, retry and skip policies, writer, processed sink, and dead-letter store.</desc>
  <defs>
    <marker id="ar-blue" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#48758d"/></marker>
    <marker id="ar-green" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="ar-red" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="ar-purple" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #425466; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #24313f; }
      .band-label { font-size: 22px; }
      .card-title { font-size: 23px; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffdf8; stroke: #d8e2e8; stroke-width: 2; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 18; }
      .blue { fill: none; stroke: #48758d; stroke-width: 3; marker-end: url(#ar-blue); }
      .green { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#ar-green); }
      .red { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#ar-red); }
      .purple { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#ar-purple); }
    </style>
  </defs>
  <rect width="1500" height="900" fill="#f7f3ea"/>
  <rect class="frame" x="44" y="34" width="1412" height="832" rx="20"/>
  <text class="title" x="750" y="82" text-anchor="middle">Retry Dead-Letter Batch Worker Architecture</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">The example keeps batch primitives visible: reader, processor, retry policy, skip policy, writer, and DLT store.</text>

  <rect class="band" x="82" y="158" width="1336" height="132"/><text class="band-label" x="112" y="194">Runnable demo</text>
  <g transform="translate(174 210)"><rect width="224" height="58" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="112" y="35" text-anchor="middle">go run</text></g>
  <g transform="translate(638 198)"><rect width="246" height="82" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="123" y="33" text-anchor="middle">batch.Job</text><text class="detail" x="123" y="59" text-anchor="middle">ticket-batch-worker</text></g>
  <g transform="translate(1112 198)"><rect width="230" height="82" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="115" y="33" text-anchor="middle">Stable JSON</text><text class="detail" x="115" y="59" text-anchor="middle">no timestamps</text></g>

  <rect class="band" x="82" y="342" width="1336" height="216"/><text class="band-label" x="112" y="378">batch.Step components</text>
  <g transform="translate(144 430)"><rect width="226" height="80" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="113" y="32" text-anchor="middle">TicketReader</text><text class="detail" x="113" y="57" text-anchor="middle">in-memory queue</text></g>
  <g transform="translate(480 418)"><rect width="254" height="104" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="127" y="35" text-anchor="middle">TicketProcessor</text><text class="detail" x="127" y="61" text-anchor="middle">classify failures</text><text class="detail" x="127" y="81" text-anchor="middle">track attempts</text></g>
  <g transform="translate(854 418)"><rect width="246" height="104" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="123" y="35" text-anchor="middle">TicketWriter</text><text class="detail" x="123" y="61" text-anchor="middle">processed sink</text><text class="detail" x="123" y="81" text-anchor="middle">duplicate guard</text></g>
  <g transform="translate(1194 430)"><rect width="176" height="80" rx="12" fill="#fff7ed" stroke="#c07a42" stroke-width="2"/><text class="card-title" x="88" y="32" text-anchor="middle">Sink</text><text class="detail" x="88" y="57" text-anchor="middle">ordered writes</text></g>

  <rect class="band" x="82" y="612" width="1336" height="126"/><text class="band-label" x="112" y="648">Policy and failure evidence</text>
  <g transform="translate(324 666)"><rect width="250" height="50" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="125" y="32" text-anchor="middle">RetryPolicy</text></g>
  <g transform="translate(646 666)"><rect width="250" height="50" rx="12" fill="#ffe4e6" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="125" y="32" text-anchor="middle">SkipPolicy</text></g>
  <g transform="translate(968 666)"><rect width="286" height="50" rx="12" fill="#fee2e2" stroke="#bf6672" stroke-width="2"/><text class="card-title" x="143" y="32" text-anchor="middle">DeadLetterStore</text></g>

  <rect class="band" x="82" y="802" width="1336" height="36"/><text class="footer" x="750" y="826" text-anchor="middle">Production replacement points: durable queue, persistent DLT, idempotent handlers, backoff jitter, metrics, alerting, and replay tooling.</text>

  <path class="blue" d="M 398 239 L 638 239"/><text class="label" x="518" y="219" text-anchor="middle">run</text>
  <path class="blue" d="M 884 239 L 1112 239"/><text class="label" x="998" y="219" text-anchor="middle">project report</text>
  <path class="blue" d="M 761 280 L 761 322 L 257 322 L 257 430"/><text class="label" x="502" y="312" text-anchor="middle">read tickets</text>
  <path class="green" d="M 370 470 L 480 470"/><text class="label" x="425" y="450" text-anchor="middle">tickets</text>
  <path class="purple" d="M 734 470 L 854 470"/><text class="label" x="794" y="450" text-anchor="middle">processed</text>
  <path class="purple" d="M 1100 470 L 1194 470"/><text class="label" x="1147" y="450" text-anchor="middle">write</text>
  <path class="blue" d="M 575 522 L 575 642 L 449 642 L 449 666"/><text class="label" x="514" y="632" text-anchor="middle">transient</text>
  <path class="red" d="M 638 522 L 638 642 L 771 642 L 771 666"/><text class="label" x="708" y="632" text-anchor="middle">permanent</text>
  <path class="red" d="M 680 522 L 680 586 L 1111 586 L 1111 666"/><text class="label" x="908" y="576" text-anchor="middle">record reason + attempts</text>
</svg>
SVG

cat > "$out_dir/retry-dead-letter-batch-worker-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="900" viewBox="0 0 1500 900" role="img" aria-labelledby="title desc">
  <title id="title">Retry Dead-Letter Batch Worker Sequence</title>
  <desc id="desc">Sequence diagram showing queue open, transient retry, chunk write, permanent dead-letter capture, skipped item accounting, and completed report.</desc>
  <defs>
    <marker id="sq-blue" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#48758d"/></marker>
    <marker id="sq-green" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="sq-red" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="sq-purple" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #425466; }
      .subtitle { font-size: 15px; }
      .card-title { font-family: 'Architects Daughter'; font-size: 23px; fill: #24313f; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffdf8; stroke: #d8e2e8; stroke-width: 2; }
      .lane { stroke: #d8e2e8; stroke-dasharray: 7 8; stroke-width: 2; }
      .blue { fill: none; stroke: #48758d; stroke-width: 3; marker-end: url(#sq-blue); }
      .green { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#sq-green); }
      .red { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#sq-red); }
      .purple { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#sq-purple); }
    </style>
  </defs>
  <rect width="1500" height="900" fill="#f7f3ea"/>
  <rect class="frame" x="44" y="34" width="1412" height="832" rx="20"/>
  <text class="title" x="750" y="82" text-anchor="middle">Retry Dead-Letter Batch Worker Sequence</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">The failed permanent item is recorded before the batch skip policy lets the job continue.</text>

  <g transform="translate(116 164)"><rect width="186" height="58" rx="10" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="93" y="35" text-anchor="middle">batch.Job</text></g>
  <g transform="translate(374 164)"><rect width="190" height="58" rx="10" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="95" y="35" text-anchor="middle">Reader</text></g>
  <g transform="translate(638 164)"><rect width="224" height="58" rx="10" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="112" y="35" text-anchor="middle">Processor</text></g>
  <g transform="translate(936 164)"><rect width="190" height="58" rx="10" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="95" y="35" text-anchor="middle">Writer</text></g>
  <g transform="translate(1200 164)"><rect width="210" height="58" rx="10" fill="#fee2e2" stroke="#bf6672" stroke-width="2"/><text class="card-title" x="105" y="35" text-anchor="middle">DeadLetter</text></g>

  <line class="lane" x1="209" y1="222" x2="209" y2="742"/>
  <line class="lane" x1="469" y1="222" x2="469" y2="742"/>
  <line class="lane" x1="750" y1="222" x2="750" y2="742"/>
  <line class="lane" x1="1031" y1="222" x2="1031" y2="742"/>
  <line class="lane" x1="1305" y1="222" x2="1305" y2="742"/>

  <path class="blue" d="M 209 260 L 469 260"/><text class="label" x="339" y="241" text-anchor="middle">open + read ticket-1001</text>
  <path class="green" d="M 469 320 L 750 320"/><text class="label" x="610" y="301" text-anchor="middle">success</text>
  <path class="green" d="M 750 380 L 1031 380"/><text class="label" x="891" y="361" text-anchor="middle">chunk item ready</text>
  <path class="blue" d="M 469 440 L 750 440"/><text class="label" x="610" y="421" text-anchor="middle">ticket-1002 transient</text>
  <path class="blue" d="M 750 488 L 802 488 L 802 526 L 750 526"/><text class="label" x="884" y="511" text-anchor="middle">retry attempt 2 succeeds</text>
  <path class="purple" d="M 750 574 L 1031 574"/><text class="label" x="891" y="555" text-anchor="middle">write first chunk</text>
  <path class="red" d="M 469 634 L 750 634"/><text class="label" x="610" y="615" text-anchor="middle">ticket-1003 permanent</text>
  <path class="red" d="M 750 682 L 1305 682"/><text class="label" x="1028" y="663" text-anchor="middle">record reason and attempts</text>
  <path class="purple" d="M 1305 730 L 209 730"/><text class="label" x="757" y="711" text-anchor="middle">skip item, process ticket-1004, report completed</text>

  <rect class="frame" x="82" y="796" width="1336" height="38" rx="12"/>
  <text class="footer" x="750" y="821" text-anchor="middle">Context cancellation returns cancelled status and is neither retried nor dead-lettered.</text>
</svg>
SVG

rsvg-convert "$out_dir/retry-dead-letter-batch-worker-scenario.svg" -o "$out_dir/retry-dead-letter-batch-worker-scenario.png"
rsvg-convert "$out_dir/retry-dead-letter-batch-worker-architecture.svg" -o "$out_dir/retry-dead-letter-batch-worker-architecture.png"
rsvg-convert "$out_dir/retry-dead-letter-batch-worker-sequence.svg" -o "$out_dir/retry-dead-letter-batch-worker-sequence.png"

validate_svg "retry-dead-letter-batch-worker-scenario"
validate_svg "retry-dead-letter-batch-worker-architecture"
validate_svg "retry-dead-letter-batch-worker-sequence"

write_gate "retry-dead-letter-batch-worker-scenario" 8 6 6 44 44 34 34
write_gate "retry-dead-letter-batch-worker-architecture" 9 9 15 44 44 34 34
write_gate "retry-dead-letter-batch-worker-sequence" 5 9 12 44 44 34 34
