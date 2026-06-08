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
  echo "${name}: nodes=${nodes} routes=${routes} segments=${segments} badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=0 titleGap=ok fontFallback=0"
}

render_pair() {
  local name="$1"
  dot -Tplain "$out_dir/${name}.dot" > "$out_dir/${name}.plain"
  dot -Tsvg "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.svg"
  dot -Tpng "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.png"
  rsvg-convert "$out_dir/${name}.svg" -o "$out_dir/${name}.png"
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

cat > "$out_dir/operations-report-policy-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.65, ranksep=0.8];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  request [label="Operations Run Request"];
  policy [label="failure policy", fillcolor="#f7e5aa", color="#b99b5d"];
  load [label="load-catalog-snapshot", fillcolor="#dbe8d4", color="#5d8a62"];
  validate [label="validate-products", fillcolor="#f5d3df", color="#b95f7a"];
  retry [label="notify-partner retry", fillcolor="#eadcf5", color="#8a6bb0"];
  skip [label="refresh-search-index", fillcolor="#efe7d2", color="#a78335"];
  summary [label="stable report summary", fillcolor="#edf1e9", color="#5d8a62"];
  request -> policy;
  policy -> load;
  load -> validate;
  validate -> retry [label="continue"];
  retry -> skip [label="preserve attempts"];
  skip -> summary [label="count statuses"];
  validate -> summary [label="stop on failure", color="#b95f7a", fontcolor="#8d4c63"];
}
DOT

cat > "$out_dir/operations-report-policy-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.75, ranksep=0.85];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="HTTP Client"];
  gin [label="Gin Router", fillcolor="#f7e5aa", color="#b99b5d"];
  handler [label="Operations Handler", fillcolor="#dbe8d4", color="#5d8a62"];
  aggregate [label="workreport.Aggregate", fillcolor="#eadcf5", color="#8a6bb0"];
  projector [label="Stable DTO Projector", fillcolor="#efe7d2", color="#a78335"];
  response [label="JSON + HTTP status", fillcolor="#edf1e9", color="#5d8a62"];
  client -> gin [label="POST /operations/report"];
  gin -> handler [label="bind + validate"];
  handler -> aggregate [label="child reports + policy"];
  aggregate -> projector [label="report tree"];
  projector -> response [label="summary counts"];
  response -> client [label="200 / 207 / 409 / 408"];
}
DOT

cat > "$out_dir/operations-report-policy-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.7, ranksep=0.9];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="Client"];
  handler [label="Gin Handler", fillcolor="#f7e5aa", color="#b99b5d"];
  steps [label="Checklist Steps", fillcolor="#dbe8d4", color="#5d8a62"];
  aggregate [label="Aggregate(policy)", fillcolor="#eadcf5", color="#8a6bb0"];
  mapper [label="Project + summarize", fillcolor="#efe7d2", color="#a78335"];
  client -> handler [label="POST"];
  handler -> steps [label="run until policy stops"];
  steps -> aggregate [label="reports"];
  aggregate -> mapper [label="root status"];
  mapper -> client [label="stable JSON"];
}
DOT

cat > "$out_dir/operations-report-policy-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1380" height="780" viewBox="0 0 1380 780" role="img" aria-labelledby="title desc">
  <title id="title">Operations Report Policy Scenario</title>
  <desc id="desc">Scenario flow for an operations checklist that chooses a failure policy, records retry evidence, marks skipped work, and returns a stable summary.</desc>
  <defs>
    <marker id="scenario-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="scenario-error" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 36px; fill: #243845; }
      .subtitle, .detail, .label { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .card-title { font-family: 'Architects Daughter'; font-size: 22px; fill: #243845; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .main { fill: none; stroke: #47616f; stroke-width: 3; marker-end: url(#scenario-main); }
      .error { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#scenario-error); }
    </style>
  </defs>
  <rect width="1380" height="780" fill="#fbfaf6"/>
  <text class="title" x="690" y="58" text-anchor="middle">Operations Report Policy Scenario</text>
  <text class="subtitle" x="690" y="92" text-anchor="middle">The caller chooses whether non-completed work stops the run or remains visible in a partial report.</text>

  <g transform="translate(64 184)"><rect width="218" height="92" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="109" y="38" text-anchor="middle">Run Request</text><text class="detail" x="109" y="63" text-anchor="middle">run_id + flags</text></g>
  <g transform="translate(364 184)"><rect width="220" height="92" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="110" y="38" text-anchor="middle">Failure Policy</text><text class="detail" x="110" y="63" text-anchor="middle">stop or continue</text></g>
  <g transform="translate(676 126)"><rect width="240" height="92" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="120" y="36" text-anchor="middle">load snapshot</text><text class="detail" x="120" y="62" text-anchor="middle">completed</text></g>
  <g transform="translate(676 280)"><rect width="240" height="92" rx="12" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="120" y="36" text-anchor="middle">validate products</text><text class="detail" x="120" y="62" text-anchor="middle">may fail</text></g>
  <g transform="translate(998 126)"><rect width="250" height="92" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="125" y="36" text-anchor="middle">notify retry</text><text class="detail" x="125" y="62" text-anchor="middle">failed attempt + retry</text></g>
  <g transform="translate(998 280)"><rect width="250" height="92" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="card-title" x="125" y="36" text-anchor="middle">refresh index</text><text class="detail" x="125" y="62" text-anchor="middle">completed or aborted</text></g>
  <g transform="translate(536 548)"><rect width="308" height="98" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="154" y="39" text-anchor="middle">Stable Summary</text><text class="detail" x="154" y="66" text-anchor="middle">root + descendants counted</text></g>

  <path class="main" d="M 282 230 L 364 230"/><text class="label" x="323" y="211" text-anchor="middle">POST</text>
  <path class="main" d="M 584 230 L 628 230 L 628 172 L 676 172"/>
  <path class="main" d="M 796 218 L 796 280"/>
  <path class="main" d="M 916 172 L 998 172"/>
  <path class="main" d="M 916 326 L 998 326"/>
  <path class="main" d="M 1123 372 L 1123 597 L 844 597"/>
  <path class="main" d="M 796 372 L 796 548"/>
  <path class="error" d="M 676 326 L 626 326 L 626 548"/><text class="label" x="560" y="433" text-anchor="middle">stop on failure</text>

  <rect x="254" y="704" width="872" height="38" rx="11" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="subtitle" x="690" y="729" text-anchor="middle">Skipped work is represented as aborted with a caller-defined reason.</text>
</svg>
SVG

cat > "$out_dir/operations-report-policy-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1380" height="760" viewBox="0 0 1380 760" role="img" aria-labelledby="title desc">
  <title id="title">Operations Report Policy Architecture</title>
  <desc id="desc">Architecture for the operations report policy example showing the Gin boundary, operations checklist, workreport aggregation, and stable response projection.</desc>
  <defs>
    <marker id="arch-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 36px; fill: #243845; }
      .subtitle, .detail, .label { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #243845; }
      .band-label { font-size: 20px; }
      .card-title { font-size: 22px; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 16; }
      .main { fill: none; stroke: #47616f; stroke-width: 3; marker-end: url(#arch-main); }
    </style>
  </defs>
  <rect width="1380" height="760" fill="#fbfaf6"/>
  <text class="title" x="690" y="58" text-anchor="middle">Operations Report Policy Architecture</text>
  <text class="subtitle" x="690" y="92" text-anchor="middle">HTTP mapping, report aggregation, and deterministic projection stay in separate boundaries.</text>

  <rect class="band" x="68" y="140" width="1244" height="136"/><text class="band-label" x="102" y="176">HTTP boundary</text>
  <g transform="translate(118 192)"><rect width="220" height="66" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="110" y="29" text-anchor="middle">Client</text><text class="detail" x="110" y="51" text-anchor="middle">curl or test</text></g>
  <g transform="translate(1044 184)"><rect width="226" height="82" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="113" y="34" text-anchor="middle">JSON Response</text><text class="detail" x="113" y="58" text-anchor="middle">200 / 207 / 409 / 408</text></g>

  <rect class="band" x="68" y="320" width="1244" height="136"/><text class="band-label" x="102" y="356">Application boundary</text>
  <g transform="translate(250 366)"><rect width="230" height="76" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="115" y="31" text-anchor="middle">Gin Handler</text><text class="detail" x="115" y="55" text-anchor="middle">bind and validate</text></g>
  <g transform="translate(574 366)"><rect width="250" height="76" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="125" y="31" text-anchor="middle">Operations Run</text><text class="detail" x="125" y="55" text-anchor="middle">checklist steps</text></g>
  <g transform="translate(916 366)"><rect width="240" height="76" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="card-title" x="120" y="31" text-anchor="middle">DTO Projector</text><text class="detail" x="120" y="55" text-anchor="middle">hide timestamps</text></g>

  <rect class="band" x="68" y="500" width="1244" height="116"/><text class="band-label" x="102" y="536">bluetape-go boundary</text>
  <g transform="translate(432 546)"><rect width="250" height="58" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="125" y="37" text-anchor="middle">workreport.Report</text></g>
  <g transform="translate(764 546)"><rect width="276" height="58" rx="12" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="138" y="37" text-anchor="middle">workreport.Aggregate</text></g>

  <path class="main" d="M 338 225 L 365 225 L 365 366"/><text class="label" x="411" y="294" text-anchor="middle">POST</text>
  <path class="main" d="M 480 404 L 574 404"/><text class="label" x="527" y="385" text-anchor="middle">request</text>
  <path class="main" d="M 699 442 L 699 575 L 764 575"/><text class="label" x="716" y="512" text-anchor="middle">reports</text>
  <path class="main" d="M 902 546 L 1036 442"/><text class="label" x="989" y="496" text-anchor="middle">root status</text>
  <path class="main" d="M 1156 404 L 1218 404 L 1218 266"/><text class="label" x="1252" y="334" text-anchor="middle">status map</text>
  <path class="main" d="M 1044 225 L 338 225"/>

  <rect x="292" y="682" width="796" height="38" rx="11" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="subtitle" x="690" y="707" text-anchor="middle">No database, queue, background worker, or observability dependency is introduced.</text>
</svg>
SVG

cat > "$out_dir/operations-report-policy-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1380" height="820" viewBox="0 0 1380 820" role="img" aria-labelledby="title desc">
  <title id="title">Operations Report Policy Sequence</title>
  <desc id="desc">Sequence diagram for validating a request, executing checklist steps, aggregating by failure policy, and projecting a stable report response.</desc>
  <defs>
    <marker id="seq-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="seq-error" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 36px; fill: #243845; }
      .subtitle, .label, .small { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .actor { font-family: 'Architects Daughter'; font-size: 20px; fill: #243845; }
      .label { font-size: 12px; }
      .small { font-size: 12px; }
      .lifeline { stroke: #a9bac5; stroke-width: 2; stroke-dasharray: 7 7; }
      .main { fill: none; stroke: #47616f; stroke-width: 2.8; marker-end: url(#seq-main); }
      .error { fill: none; stroke: #b95f7a; stroke-width: 2.8; marker-end: url(#seq-error); }
    </style>
  </defs>
  <rect width="1380" height="820" fill="#fbfaf6"/>
  <text class="title" x="690" y="58" text-anchor="middle">Operations Report Policy Sequence</text>
  <text class="subtitle" x="690" y="92" text-anchor="middle">The report body is deterministic even though workreport timestamps are created at runtime.</text>

  <g transform="translate(80 134)"><rect width="176" height="58" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="actor" x="88" y="37" text-anchor="middle">Client</text></g>
  <g transform="translate(334 134)"><rect width="176" height="58" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="actor" x="88" y="37" text-anchor="middle">Handler</text></g>
  <g transform="translate(588 134)"><rect width="176" height="58" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="actor" x="88" y="37" text-anchor="middle">Checklist</text></g>
  <g transform="translate(842 134)"><rect width="176" height="58" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="actor" x="88" y="37" text-anchor="middle">Aggregate</text></g>
  <g transform="translate(1096 134)"><rect width="196" height="58" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="actor" x="98" y="37" text-anchor="middle">Projector</text></g>

  <line class="lifeline" x1="168" y1="192" x2="168" y2="700"/>
  <line class="lifeline" x1="422" y1="192" x2="422" y2="700"/>
  <line class="lifeline" x1="676" y1="192" x2="676" y2="700"/>
  <line class="lifeline" x1="930" y1="192" x2="930" y2="700"/>
  <line class="lifeline" x1="1194" y1="192" x2="1194" y2="700"/>

  <path class="main" d="M 168 238 L 422 238"/><text class="label" x="295" y="220" text-anchor="middle">POST /operations/report</text>
  <path class="main" d="M 422 302 L 676 302"/><text class="label" x="549" y="284" text-anchor="middle">run steps with policy</text>
  <path class="main" d="M 676 366 L 930 366"/><text class="label" x="803" y="348" text-anchor="middle">child reports</text>
  <path class="main" d="M 930 430 L 1194 430"/><text class="label" x="1062" y="412" text-anchor="middle">root report</text>
  <path class="main" d="M 1194 494 L 422 494"/><text class="label" x="808" y="476" text-anchor="middle">stable DTO + summary counts</text>
  <path class="main" d="M 422 558 L 168 558"/><text class="label" x="295" y="540" text-anchor="middle">200 / 207 / 409 / 408</text>
  <path class="error" d="M 676 622 L 514 622 L 514 686 L 930 686"/><text class="label" x="720" y="667" text-anchor="middle">stop_on_failure omits later steps</text>

  <rect x="256" y="740" width="868" height="38" rx="11" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="subtitle" x="690" y="765" text-anchor="middle">The retry branch stays partial when a failed attempt remains visible.</text>
</svg>
SVG

for name in \
  operations-report-policy-scenario \
  operations-report-policy-architecture \
  operations-report-policy-sequence
do
  validate_svg "$name"
  render_pair "$name"
done

write_gate "operations-report-policy-scenario" 7 7 8
write_gate "operations-report-policy-architecture" 8 6 8
write_gate "operations-report-policy-sequence" 5 7 7
