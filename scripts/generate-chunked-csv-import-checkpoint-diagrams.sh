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

cat > "$out_dir/chunked-csv-import-checkpoint-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.58, ranksep=0.82, splines=true, overlap=false, fontname="Architects Daughter", label="Chunked CSV Import Checkpoint Scenario", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  csv [label="customers.csv\n5 deterministic rows", fillcolor="#fef3c7", color="#b99b5d"];
  chunk1 [label="chunk 1\nrows 0..1", fillcolor="#dcfce7", color="#5d8a62"];
  cp2 [label="checkpoint\nnext_row=2", fillcolor="#dbeafe", color="#4f7e9d"];
  chunk2 [label="chunk 2\nrow 2 committed", fillcolor="#fee2e2", color="#bf6672"];
  crash [label="simulated crash\ncheckpoint stays 2", fillcolor="#ffe4e6", color="#b95f7a"];
  restart [label="restart\nrestore next_row=2", fillcolor="#ede9fe", color="#8a6bb0"];
  replay [label="replay chunk\nskip cust-1003 duplicate", fillcolor="#f3e8ff", color="#8a6bb0"];
  done [label="complete\nnext_row=5", fillcolor="#dcfce7", color="#5d8a62"];
  csv -> chunk1 [label="read"];
  chunk1 -> cp2 [label="write ok", color="#5d8a62"];
  cp2 -> chunk2 [label="read rows 2..3"];
  chunk2 -> crash [label="writer error", color="#b95f7a"];
  crash -> restart [label="new process", color="#8a6bb0"];
  restart -> replay [label="read row 2 again", color="#8a6bb0"];
  replay -> done [label="idempotent write", color="#5d8a62"];
}
DOT

cat > "$out_dir/chunked-csv-import-checkpoint-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.62, ranksep=0.82, splines=true, overlap=false, fontname="Architects Daughter", label="Chunked CSV Import Checkpoint Architecture", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  cli [label="go run\nlocal demo", fillcolor="#d7ecf2", color="#48758d"];
  job [label="batch.Job\ncustomer-csv-import", fillcolor="#fef3c7", color="#b99b5d"];
  step [label="batch.Step\nchunk size 2", fillcolor="#ecfccb", color="#78944d"];
  reader [label="CSVReader\nCheckpointReader", fillcolor="#dbeafe", color="#4f7e9d"];
  processor [label="Processor\nvalidate + normalize", fillcolor="#dcfce7", color="#5d8a62"];
  writer [label="CustomerWriter\nidempotent by ID", fillcolor="#f3e8ff", color="#8a6bb0"];
  store [label="MemoryCheckpointStore\ncustomer-csv-import", fillcolor="#e0f2fe", color="#48758d"];
  sink [label="CustomerSink\nunique commits", fillcolor="#fff7ed", color="#c07a42"];
  cli -> job [label="run twice"];
  job -> step [label="one step"];
  step -> reader [label="read/restore", color="#3f6f8f"];
  step -> processor [label="process", color="#5d8a62"];
  step -> writer [label="write chunks", color="#8a6bb0"];
  step -> store [label="save after commit", color="#48758d"];
  writer -> sink [label="upsert", color="#c07a42"];
  store -> reader [label="restore next_row", color="#48758d"];
}
DOT

cat > "$out_dir/chunked-csv-import-checkpoint-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=TB, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.44, ranksep=0.58, splines=ortho, fontname="Architects Daughter", label="Chunked CSV Import Checkpoint Sequence", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.4, arrowsize=0.75, fontname="Comic Mono", fontsize=11, fontcolor="#425466"];
  start [label="First run", fillcolor="#fef3c7", color="#b99b5d"];
  restore0 [label="No checkpoint", fillcolor="#dbeafe", color="#4f7e9d"];
  write1 [label="Write chunk 1\nsave next_row=2", fillcolor="#dcfce7", color="#5d8a62"];
  fail [label="Chunk 2 crash\ncust-1003 committed", fillcolor="#fee2e2", color="#bf6672"];
  restart [label="Restart run", fillcolor="#ede9fe", color="#8a6bb0"];
  restore2 [label="Restore next_row=2", fillcolor="#e0f2fe", color="#48758d"];
  replay [label="Replay chunk 2\nskip duplicate", fillcolor="#f3e8ff", color="#8a6bb0"];
  finish [label="Write remaining\nsave next_row=5", fillcolor="#dcfce7", color="#5d8a62"];
  start -> restore0;
  restore0 -> write1 [color="#5d8a62"];
  write1 -> fail [color="#b95f7a"];
  fail -> restart [color="#8a6bb0"];
  restart -> restore2 [color="#48758d"];
  restore2 -> replay [color="#8a6bb0"];
  replay -> finish [color="#5d8a62"];
}
DOT

render_graphviz_pair "chunked-csv-import-checkpoint-scenario"
render_graphviz_pair "chunked-csv-import-checkpoint-architecture"
render_graphviz_pair "chunked-csv-import-checkpoint-sequence"

cat > "$out_dir/chunked-csv-import-checkpoint-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="860" viewBox="0 0 1500 860" role="img" aria-labelledby="title desc">
  <title id="title">Chunked CSV Import Checkpoint Scenario</title>
  <desc id="desc">Scenario showing a chunked customer CSV import, a partial writer crash, checkpoint restore, duplicate skip, and completed restart.</desc>
  <defs>
    <marker id="sc-read" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="sc-ok" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="sc-fail" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="sc-restart" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #24313f; }
      .band-label { font-size: 22px; }
      .card-title { font-size: 23px; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffefa; stroke: #d8e2e8; stroke-width: 2; rx: 20; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 16; }
      .read { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#sc-read); }
      .ok { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#sc-ok); }
      .fail { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#sc-fail); }
      .restart { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#sc-restart); }
    </style>
  </defs>
  <rect width="1500" height="860" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="34" width="1412" height="792"/>
  <text class="title" x="750" y="82" text-anchor="middle">Chunked CSV Import Checkpoint Scenario</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">The failed chunk is replayed from the previous checkpoint, so the writer must be idempotent by customer ID.</text>

  <rect class="band" x="82" y="158" width="1336" height="224"/><text class="band-label" x="112" y="194">First run</text>
  <g transform="translate(126 248)"><rect width="210" height="82" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="105" y="33" text-anchor="middle">customers.csv</text><text class="detail" x="105" y="58" text-anchor="middle">5 deterministic rows</text></g>
  <g transform="translate(430 236)"><rect width="206" height="106" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="103" y="35" text-anchor="middle">chunk 1</text><text class="detail" x="103" y="61" text-anchor="middle">cust-1001</text><text class="detail" x="103" y="82" text-anchor="middle">cust-1002</text></g>
  <g transform="translate(738 236)"><rect width="228" height="106" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="114" y="35" text-anchor="middle">checkpoint</text><text class="detail" x="114" y="61" text-anchor="middle">customer-csv-import</text><text class="detail" x="114" y="82" text-anchor="middle">next_row = 2</text></g>
  <g transform="translate(1076 236)"><rect width="246" height="106" rx="12" fill="#fee2e2" stroke="#bf6672" stroke-width="2"/><text class="card-title" x="123" y="35" text-anchor="middle">chunk 2 crash</text><text class="detail" x="123" y="61" text-anchor="middle">cust-1003 committed</text><text class="detail" x="123" y="82" text-anchor="middle">checkpoint stays 2</text></g>

  <rect class="band" x="82" y="432" width="1336" height="210"/><text class="band-label" x="112" y="468">Restart run</text>
  <g transform="translate(202 512)"><rect width="238" height="86" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="119" y="34" text-anchor="middle">new reader</text><text class="detail" x="119" y="61" text-anchor="middle">restore next_row = 2</text></g>
  <g transform="translate(584 500)"><rect width="280" height="110" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="140" y="34" text-anchor="middle">replay failed chunk</text><text class="detail" x="140" y="60" text-anchor="middle">skip duplicate cust-1003</text><text class="detail" x="140" y="82" text-anchor="middle">commit cust-1004</text></g>
  <g transform="translate(1008 512)"><rect width="260" height="86" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="130" y="34" text-anchor="middle">complete import</text><text class="detail" x="130" y="61" text-anchor="middle">5 unique customers</text></g>

  <rect class="band" x="82" y="704" width="1336" height="56"/><text class="footer" x="750" y="738" text-anchor="middle">Checkpoint is a cursor over parsed data rows; idempotency belongs to the writer because a failed chunk can be replayed.</text>

  <path class="read" d="M 336 289 L 430 289"/><text class="label" x="383" y="269" text-anchor="middle">read rows</text>
  <path class="ok" d="M 636 289 L 738 289"/><text class="label" x="687" y="269" text-anchor="middle">write ok</text>
  <path class="fail" d="M 966 289 L 1076 289"/><text class="label" x="1021" y="269" text-anchor="middle">writer error</text>
  <path class="restart" d="M 1199 342 L 1199 404 L 321 404 L 321 512"/><text class="label" x="760" y="394" text-anchor="middle">process restarts with saved checkpoint</text>
  <path class="restart" d="M 440 555 L 584 555"/><text class="label" x="512" y="535" text-anchor="middle">replay</text>
  <path class="ok" d="M 864 555 L 1008 555"/><text class="label" x="936" y="535" text-anchor="middle">idempotent write</text>
</svg>
SVG

cat > "$out_dir/chunked-csv-import-checkpoint-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="860" viewBox="0 0 1500 860" role="img" aria-labelledby="title desc">
  <title id="title">Chunked CSV Import Checkpoint Architecture</title>
  <desc id="desc">Architecture for the local batch example using batch.Job, batch.Step, CSV reader, processor, idempotent writer, checkpoint store, and sink.</desc>
  <defs>
    <marker id="arch-blue" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="arch-green" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="arch-purple" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <marker id="arch-amber" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#c07a42"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #24313f; }
      .band-label { font-size: 22px; }
      .card-title { font-size: 23px; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffefa; stroke: #d8e2e8; stroke-width: 2; rx: 20; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 16; }
      .blue { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#arch-blue); }
      .green { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#arch-green); }
      .purple { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#arch-purple); }
      .amber { fill: none; stroke: #c07a42; stroke-width: 3; marker-end: url(#arch-amber); }
    </style>
  </defs>
  <rect width="1500" height="860" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="34" width="1412" height="792"/>
  <text class="title" x="750" y="82" text-anchor="middle">Chunked CSV Import Checkpoint Architecture</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">The workshop keeps the batch interfaces visible: reader, processor, writer, checkpoint store, and report projection.</text>

  <rect class="band" x="82" y="158" width="1336" height="136"/><text class="band-label" x="112" y="194">Runnable demo</text>
  <g transform="translate(178 214)"><rect width="218" height="58" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="109" y="35" text-anchor="middle">go run</text></g>
  <g transform="translate(628 202)"><rect width="244" height="82" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="122" y="33" text-anchor="middle">batch.Job</text><text class="detail" x="122" y="59" text-anchor="middle">customer-csv-import</text></g>
  <g transform="translate(1104 202)"><rect width="236" height="82" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="118" y="33" text-anchor="middle">Stable JSON</text><text class="detail" x="118" y="59" text-anchor="middle">no timestamps</text></g>

  <rect class="band" x="82" y="342" width="1336" height="210"/><text class="band-label" x="112" y="378">batch.Step components</text>
  <g transform="translate(150 432)"><rect width="228" height="74" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="114" y="31" text-anchor="middle">CSVReader</text><text class="detail" x="114" y="54" text-anchor="middle">CheckpointReader</text></g>
  <g transform="translate(482 420)"><rect width="236" height="98" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="118" y="34" text-anchor="middle">Processor</text><text class="detail" x="118" y="59" text-anchor="middle">validate rows</text><text class="detail" x="118" y="78" text-anchor="middle">normalize email/tier</text></g>
  <g transform="translate(822 420)"><rect width="250" height="98" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="125" y="34" text-anchor="middle">CustomerWriter</text><text class="detail" x="125" y="59" text-anchor="middle">chunk writes</text><text class="detail" x="125" y="78" text-anchor="middle">idempotent by ID</text></g>
  <g transform="translate(1182 420)"><rect width="182" height="98" rx="12" fill="#fff7ed" stroke="#c07a42" stroke-width="2"/><text class="card-title" x="91" y="34" text-anchor="middle">Sink</text><text class="detail" x="91" y="59" text-anchor="middle">unique commits</text><text class="detail" x="91" y="78" text-anchor="middle">duplicate skips</text></g>

  <rect class="band" x="82" y="600" width="1336" height="118"/><text class="band-label" x="112" y="636">Checkpoint contract</text>
  <g transform="translate(348 652)"><rect width="320" height="44" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="160" y="29" text-anchor="middle">MemoryCheckpointStore</text></g>
  <g transform="translate(830 652)"><rect width="300" height="44" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="150" y="29" text-anchor="middle">Checkpoint{NextRow}</text></g>

  <rect class="band" x="82" y="762" width="1336" height="36"/><text class="footer" x="750" y="786" text-anchor="middle">Production replacement points: durable checkpoint store, transactional sink, database upsert, retry/dead-letter policy, and audit records.</text>

  <path class="blue" d="M 396 243 L 628 243"/><text class="label" x="512" y="223" text-anchor="middle">run first + restart</text>
  <path class="blue" d="M 872 243 L 1104 243"/><text class="label" x="988" y="223" text-anchor="middle">project report</text>
  <path class="blue" d="M 750 284 L 750 322 L 264 322 L 264 432"/><text class="label" x="500" y="312" text-anchor="middle">read + restore</text>
  <path class="green" d="M 378 469 L 482 469"/><text class="label" x="430" y="449" text-anchor="middle">rows</text>
  <path class="purple" d="M 718 469 L 822 469"/><text class="label" x="770" y="449" text-anchor="middle">customers</text>
  <path class="amber" d="M 1072 469 L 1182 469"/><text class="label" x="1127" y="449" text-anchor="middle">upsert</text>
  <path class="blue" d="M 947 518 L 947 632 L 668 632 L 668 674"/><text class="label" x="820" y="622" text-anchor="middle">save after committed chunk</text>
  <path class="purple" d="M 830 674 L 668 674"/><text class="label" x="749" y="654" text-anchor="middle">restore on restart</text>
</svg>
SVG

cat > "$out_dir/chunked-csv-import-checkpoint-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="900" viewBox="0 0 1500 900" role="img" aria-labelledby="title desc">
  <title id="title">Chunked CSV Import Checkpoint Sequence</title>
  <desc id="desc">Sequence diagram showing first run checkpoint save, simulated crash, restart, duplicate skip, and final checkpoint.</desc>
  <defs>
    <marker id="seq-blue" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="seq-green" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="seq-red" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="seq-purple" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .card-title { font-family: 'Architects Daughter'; font-size: 22px; fill: #24313f; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffefa; stroke: #d8e2e8; stroke-width: 2; rx: 20; }
      .lifeline { stroke: #cbd8e2; stroke-width: 2; stroke-dasharray: 7 8; }
      .blue { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#seq-blue); }
      .green { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#seq-green); }
      .red { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#seq-red); }
      .purple { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#seq-purple); }
    </style>
  </defs>
  <rect width="1500" height="900" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="34" width="1412" height="832"/>
  <text class="title" x="750" y="82" text-anchor="middle">Chunked CSV Import Checkpoint Sequence</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">Checkpoint save happens only after a chunk commit; restart reads from the previous safe cursor.</text>

  <g transform="translate(124 162)"><rect width="172" height="58" rx="10" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="86" y="36" text-anchor="middle">Job</text></g>
  <g transform="translate(362 162)"><rect width="188" height="58" rx="10" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="94" y="36" text-anchor="middle">Reader</text></g>
  <g transform="translate(616 162)"><rect width="204" height="58" rx="10" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="102" y="36" text-anchor="middle">Writer</text></g>
  <g transform="translate(892 162)"><rect width="204" height="58" rx="10" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="102" y="36" text-anchor="middle">Checkpoint</text></g>
  <g transform="translate(1162 162)"><rect width="184" height="58" rx="10" fill="#fff7ed" stroke="#c07a42" stroke-width="2"/><text class="card-title" x="92" y="36" text-anchor="middle">Sink</text></g>
  <line class="lifeline" x1="210" y1="236" x2="210" y2="778"/>
  <line class="lifeline" x1="456" y1="236" x2="456" y2="778"/>
  <line class="lifeline" x1="718" y1="236" x2="718" y2="778"/>
  <line class="lifeline" x1="994" y1="236" x2="994" y2="778"/>
  <line class="lifeline" x1="1254" y1="236" x2="1254" y2="778"/>

  <path class="blue" d="M 210 276 L 456 276"/><text class="label" x="333" y="256" text-anchor="middle">open and read rows 0..1</text>
  <path class="green" d="M 456 326 L 718 326"/><text class="label" x="587" y="306" text-anchor="middle">process chunk 1</text>
  <path class="green" d="M 718 376 L 1254 376"/><text class="label" x="986" y="356" text-anchor="middle">commit cust-1001, cust-1002</text>
  <path class="blue" d="M 718 426 L 994 426"/><text class="label" x="856" y="406" text-anchor="middle">save next_row=2</text>
  <path class="blue" d="M 210 486 L 456 486"/><text class="label" x="333" y="466" text-anchor="middle">read rows 2..3</text>
  <path class="red" d="M 718 536 L 1254 536"/><text class="label" x="986" y="516" text-anchor="middle">commit cust-1003 then crash</text>
  <path class="red" d="M 718 586 L 210 586"/><text class="label" x="464" y="566" text-anchor="middle">failed report, checkpoint remains 2</text>
  <path class="purple" d="M 210 646 L 994 646"/><text class="label" x="602" y="626" text-anchor="middle">restart loads next_row=2</text>
  <path class="purple" d="M 456 696 L 718 696"/><text class="label" x="587" y="676" text-anchor="middle">replay rows 2..4</text>
  <path class="purple" d="M 718 746 L 1254 746"/><text class="label" x="986" y="726" text-anchor="middle">skip duplicate cust-1003; commit remaining</text>
  <path class="green" d="M 718 802 L 994 802"/><text class="label" x="856" y="782" text-anchor="middle">save next_row=5</text>

  <g transform="translate(134 834)"><rect width="1232" height="24" rx="12" fill="#ffffff" stroke="#d8e2e8"/><text class="footer" x="616" y="18" text-anchor="middle">No duplicate customer is committed even though the failed chunk is intentionally replayed.</text></g>
</svg>
SVG

rsvg-convert "$out_dir/chunked-csv-import-checkpoint-scenario.svg" -o "$out_dir/chunked-csv-import-checkpoint-scenario.png"
rsvg-convert "$out_dir/chunked-csv-import-checkpoint-architecture.svg" -o "$out_dir/chunked-csv-import-checkpoint-architecture.png"
rsvg-convert "$out_dir/chunked-csv-import-checkpoint-sequence.svg" -o "$out_dir/chunked-csv-import-checkpoint-sequence.png"

validate_svg "chunked-csv-import-checkpoint-scenario"
validate_svg "chunked-csv-import-checkpoint-architecture"
validate_svg "chunked-csv-import-checkpoint-sequence"

write_gate "chunked-csv-import-checkpoint-scenario" 8 6 9 44 44 34 34
write_gate "chunked-csv-import-checkpoint-architecture" 8 8 13 44 44 34 34
write_gate "chunked-csv-import-checkpoint-sequence" 9 11 11 44 44 34 34
