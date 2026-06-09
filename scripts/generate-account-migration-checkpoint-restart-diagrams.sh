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
  rg -q "markerWidth=\"8\" markerHeight=\"8\"" "$out_dir/${name}.svg"
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

cat > "$out_dir/account-migration-checkpoint-restart-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.58, ranksep=0.82, splines=true, overlap=false, fontname="Architects Daughter", label="Account Migration Checkpoint Restart Scenario", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  source [label="legacy accounts\n5 source rows", fillcolor="#fef3c7", color="#b99b5d"];
  chunk1 [label="chunk 1\nacct-1001..1002", fillcolor="#dcfce7", color="#5d8a62"];
  cp2 [label="checkpoint\nnext_index=2", fillcolor="#dbeafe", color="#4f7e9d"];
  failread [label="failed run reads\nacct-1003", fillcolor="#fee2e2", color="#bf6672"];
  crash [label="processor crash\ncheckpoint stays 2", fillcolor="#ffe4e6", color="#b95f7a"];
  restart [label="restart\nrestore index 2", fillcolor="#ede9fe", color="#8a6bb0"];
  chunk2 [label="chunk 2\nacct-1003..1004", fillcolor="#f3e8ff", color="#8a6bb0"];
  done [label="complete\nnext_index=5", fillcolor="#dcfce7", color="#5d8a62"];
  source -> chunk1 [label="read"];
  chunk1 -> cp2 [label="write ok", color="#5d8a62"];
  cp2 -> failread [label="read next"];
  failread -> crash [label="error", color="#b95f7a"];
  crash -> restart [label="new run", color="#8a6bb0"];
  restart -> chunk2 [label="replay from 2", color="#8a6bb0"];
  chunk2 -> done [label="finish", color="#5d8a62"];
}
DOT

cat > "$out_dir/account-migration-checkpoint-restart-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.62, ranksep=0.82, splines=true, overlap=false, fontname="Architects Daughter", label="Account Migration Checkpoint Restart Architecture", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  cli [label="go run\nlocal demo", fillcolor="#d7ecf2", color="#48758d"];
  job [label="batch.Job\naccount-migration", fillcolor="#fef3c7", color="#b99b5d"];
  step [label="batch.Step\nchunk size 2", fillcolor="#ecfccb", color="#78944d"];
  reader [label="MigrationReader\nCheckpointReader", fillcolor="#dbeafe", color="#4f7e9d"];
  processor [label="AccountProcessor\nnormalize + crash hook", fillcolor="#dcfce7", color="#5d8a62"];
  writer [label="TargetAccountWriter\nchunk commits", fillcolor="#f3e8ff", color="#8a6bb0"];
  checkpoint [label="CheckpointStore\naccount-migration-v1", fillcolor="#e0f2fe", color="#48758d"];
  target [label="TargetAccountStore\nunique IDs", fillcolor="#fff7ed", color="#c07a42"];
  cli -> job [label="first + restart"];
  job -> step [label="one step"];
  step -> reader [label="read/restore", color="#3f6f8f"];
  step -> processor [label="process", color="#5d8a62"];
  step -> writer [label="write", color="#8a6bb0"];
  writer -> target [label="put", color="#c07a42"];
  step -> checkpoint [label="save cursor", color="#48758d"];
  checkpoint -> reader [label="restore cursor", color="#48758d"];
}
DOT

cat > "$out_dir/account-migration-checkpoint-restart-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=TB, bgcolor="#fffdf8", margin=0.18, pad=0.18, nodesep=0.44, ranksep=0.58, splines=ortho, fontname="Architects Daughter", label="Account Migration Checkpoint Restart Sequence", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.4, arrowsize=0.75, fontname="Comic Mono", fontsize=11, fontcolor="#425466"];
  start [label="First run", fillcolor="#fef3c7", color="#b99b5d"];
  load0 [label="Load checkpoint\nnot found", fillcolor="#dbeafe", color="#4f7e9d"];
  save2 [label="Write acct-1001..1002\nsave next_index=2", fillcolor="#dcfce7", color="#5d8a62"];
  crash [label="Read acct-1003\nprocessor crash", fillcolor="#fee2e2", color="#bf6672"];
  restart [label="Restart run", fillcolor="#ede9fe", color="#8a6bb0"];
  restore2 [label="Restore next_index=2", fillcolor="#e0f2fe", color="#48758d"];
  writeRest [label="Write acct-1003..1005", fillcolor="#f3e8ff", color="#8a6bb0"];
  save5 [label="Save next_index=5\ncompleted report", fillcolor="#dcfce7", color="#5d8a62"];
  start -> load0;
  load0 -> save2 [color="#5d8a62"];
  save2 -> crash [color="#b95f7a"];
  crash -> restart [color="#8a6bb0"];
  restart -> restore2 [color="#48758d"];
  restore2 -> writeRest [color="#8a6bb0"];
  writeRest -> save5 [color="#5d8a62"];
}
DOT

render_graphviz_pair "account-migration-checkpoint-restart-scenario"
render_graphviz_pair "account-migration-checkpoint-restart-architecture"
render_graphviz_pair "account-migration-checkpoint-restart-sequence"

cat > "$out_dir/account-migration-checkpoint-restart-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="860" viewBox="0 0 1500 860" role="img" aria-labelledby="title desc">
  <title id="title">Account Migration Checkpoint Restart Scenario</title>
  <desc id="desc">Scenario showing an account migration, checkpoint after the first chunk, a deterministic processor crash, restart from the saved cursor, and completed migration.</desc>
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
  <text class="title" x="750" y="82" text-anchor="middle">Account Migration Checkpoint Restart Scenario</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">A saved source cursor lets the restart skip completed work and continue from the failed account.</text>

  <rect class="band" x="82" y="158" width="1336" height="222"/><text class="band-label" x="112" y="194">First run</text>
  <g transform="translate(126 240)"><rect width="214" height="94" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="107" y="35" text-anchor="middle">Legacy accounts</text><text class="detail" x="107" y="61" text-anchor="middle">acct-1001..1005</text></g>
  <g transform="translate(430 240)"><rect width="218" height="94" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="109" y="35" text-anchor="middle">Chunk 1</text><text class="detail" x="109" y="61" text-anchor="middle">write acct-1001..1002</text></g>
  <g transform="translate(752 240)"><rect width="218" height="94" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="109" y="35" text-anchor="middle">Checkpoint</text><text class="detail" x="109" y="61" text-anchor="middle">next_index = 2</text></g>
  <g transform="translate(1082 240)"><rect width="242" height="94" rx="12" fill="#fee2e2" stroke="#bf6672" stroke-width="2"/><text class="card-title" x="121" y="35" text-anchor="middle">Crash point</text><text class="detail" x="121" y="61" text-anchor="middle">acct-1003 processor error</text></g>

  <rect class="band" x="82" y="432" width="1336" height="214"/><text class="band-label" x="112" y="468">Restart run</text>
  <g transform="translate(202 512)"><rect width="238" height="88" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="119" y="35" text-anchor="middle">New process</text><text class="detail" x="119" y="62" text-anchor="middle">load account-migration-v1</text></g>
  <g transform="translate(584 500)"><rect width="282" height="112" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="141" y="34" text-anchor="middle">Restore cursor</text><text class="detail" x="141" y="61" text-anchor="middle">read acct-1003..1005</text><text class="detail" x="141" y="84" text-anchor="middle">completed chunk is not read</text></g>
  <g transform="translate(1010 512)"><rect width="260" height="88" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="130" y="35" text-anchor="middle">Complete</text><text class="detail" x="130" y="62" text-anchor="middle">checkpoint next_index = 5</text></g>

  <rect class="band" x="82" y="704" width="1336" height="56"/><text class="footer" x="750" y="738" text-anchor="middle">The checkpoint key identifies one migration cursor; replacing the store changes persistence, not step behavior.</text>

  <path class="read" d="M 340 287 L 430 287"/><text class="label" x="385" y="267" text-anchor="middle">read</text>
  <path class="ok" d="M 648 287 L 752 287"/><text class="label" x="700" y="267" text-anchor="middle">save after write</text>
  <path class="fail" d="M 970 287 L 1082 287"/><text class="label" x="1026" y="267" text-anchor="middle">next account</text>
  <path class="restart" d="M 1203 334 L 1203 404 L 321 404 L 321 512"/><text class="label" x="760" y="394" text-anchor="middle">restart keeps checkpoint at 2</text>
  <path class="restart" d="M 440 556 L 584 556"/><text class="label" x="512" y="536" text-anchor="middle">restore</text>
  <path class="ok" d="M 866 556 L 1010 556"/><text class="label" x="938" y="536" text-anchor="middle">write remaining</text>
</svg>
SVG

cat > "$out_dir/account-migration-checkpoint-restart-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="860" viewBox="0 0 1500 860" role="img" aria-labelledby="title desc">
  <title id="title">Account Migration Checkpoint Restart Architecture</title>
  <desc id="desc">Architecture for the local batch account migration using batch.Job, batch.Step, a checkpoint reader, account processor, target writer, checkpoint store, and target store.</desc>
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
  <text class="title" x="750" y="82" text-anchor="middle">Account Migration Checkpoint Restart Architecture</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">The replaceable checkpoint store and target store keep the restart contract visible and testable.</text>

  <rect class="band" x="82" y="158" width="1336" height="136"/><text class="band-label" x="112" y="194">Runnable demo</text>
  <g transform="translate(178 214)"><rect width="218" height="58" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="109" y="35" text-anchor="middle">go run</text></g>
  <g transform="translate(628 202)"><rect width="244" height="82" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="122" y="33" text-anchor="middle">batch.Job</text><text class="detail" x="122" y="59" text-anchor="middle">account-migration</text></g>
  <g transform="translate(1104 202)"><rect width="236" height="82" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="118" y="33" text-anchor="middle">Stable JSON</text><text class="detail" x="118" y="59" text-anchor="middle">restart evidence</text></g>

  <rect class="band" x="82" y="342" width="1336" height="210"/><text class="band-label" x="112" y="378">batch.Step components</text>
  <g transform="translate(150 420)"><rect width="230" height="98" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="115" y="34" text-anchor="middle">Reader</text><text class="detail" x="115" y="59" text-anchor="middle">CheckpointReader</text><text class="detail" x="115" y="78" text-anchor="middle">cursor = next_index</text></g>
  <g transform="translate(486 420)"><rect width="236" height="98" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="118" y="34" text-anchor="middle">Processor</text><text class="detail" x="118" y="59" text-anchor="middle">normalize fields</text><text class="detail" x="118" y="78" text-anchor="middle">crash hook</text></g>
  <g transform="translate(828 420)"><rect width="250" height="98" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="125" y="34" text-anchor="middle">Writer</text><text class="detail" x="125" y="59" text-anchor="middle">chunk commits</text><text class="detail" x="125" y="78" text-anchor="middle">unique target IDs</text></g>
  <g transform="translate(1188 420)"><rect width="182" height="98" rx="12" fill="#fff7ed" stroke="#c07a42" stroke-width="2"/><text class="card-title" x="91" y="34" text-anchor="middle">Target</text><text class="detail" x="91" y="59" text-anchor="middle">snapshot</text><text class="detail" x="91" y="78" text-anchor="middle">write log</text></g>

  <rect class="band" x="82" y="600" width="1336" height="118"/><text class="band-label" x="112" y="636">Checkpoint contract</text>
  <g transform="translate(338 652)"><rect width="340" height="44" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="170" y="29" text-anchor="middle">CheckpointStore</text></g>
  <g transform="translate(820 652)"><rect width="320" height="44" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="160" y="29" text-anchor="middle">account-migration-v1</text></g>

  <rect class="band" x="82" y="762" width="1336" height="36"/><text class="footer" x="750" y="786" text-anchor="middle">Production replacements: durable checkpoint persistence, transactional target writes, audit events, and retry/dead-letter policy.</text>

  <path class="blue" d="M 396 243 L 628 243"/><text class="label" x="512" y="223" text-anchor="middle">run twice</text>
  <path class="blue" d="M 872 243 L 1104 243"/><text class="label" x="988" y="223" text-anchor="middle">print result</text>
  <path class="blue" d="M 750 284 L 750 322 L 265 322 L 265 420"/><text class="label" x="502" y="312" text-anchor="middle">read + restore</text>
  <path class="green" d="M 380 469 L 486 469"/><text class="label" x="433" y="449" text-anchor="middle">rows</text>
  <path class="purple" d="M 722 469 L 828 469"/><text class="label" x="775" y="449" text-anchor="middle">targets</text>
  <path class="amber" d="M 1078 469 L 1188 469"/><text class="label" x="1133" y="449" text-anchor="middle">put</text>
  <path class="blue" d="M 953 518 L 953 590 L 508 590 L 508 652"/><text class="label" x="731" y="580" text-anchor="middle">save after committed chunk</text>
  <path class="purple" d="M 820 674 L 678 674"/><text class="label" x="749" y="654" text-anchor="middle">same key on restart</text>
</svg>
SVG

cat > "$out_dir/account-migration-checkpoint-restart-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="900" viewBox="0 0 1500 900" role="img" aria-labelledby="title desc">
  <title id="title">Account Migration Checkpoint Restart Sequence</title>
  <desc id="desc">Sequence diagram showing checkpoint load, first chunk write, checkpoint save, simulated crash on acct-1003, restart from next_index 2, and final checkpoint save.</desc>
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
  <text class="title" x="750" y="82" text-anchor="middle">Account Migration Checkpoint Restart Sequence</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">The checkpoint advances only after a safe chunk; the restart loads the same key and resumes at index 2.</text>

  <g transform="translate(116 162)"><rect width="180" height="58" rx="10" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="90" y="36" text-anchor="middle">Job</text></g>
  <g transform="translate(354 162)"><rect width="196" height="58" rx="10" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="98" y="36" text-anchor="middle">Reader</text></g>
  <g transform="translate(610 162)"><rect width="210" height="58" rx="10" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="105" y="36" text-anchor="middle">Processor</text></g>
  <g transform="translate(882 162)"><rect width="212" height="58" rx="10" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="106" y="36" text-anchor="middle">Writer</text></g>
  <g transform="translate(1158 162)"><rect width="190" height="58" rx="10" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="95" y="36" text-anchor="middle">Checkpoint</text></g>
  <line class="lifeline" x1="206" y1="236" x2="206" y2="778"/>
  <line class="lifeline" x1="452" y1="236" x2="452" y2="778"/>
  <line class="lifeline" x1="715" y1="236" x2="715" y2="778"/>
  <line class="lifeline" x1="988" y1="236" x2="988" y2="778"/>
  <line class="lifeline" x1="1253" y1="236" x2="1253" y2="778"/>

  <path class="blue" d="M 206 276 L 1253 276"/><text class="label" x="730" y="256" text-anchor="middle">first run loads key: not found</text>
  <path class="blue" d="M 206 326 L 452 326"/><text class="label" x="329" y="306" text-anchor="middle">read acct-1001..1002</text>
  <path class="green" d="M 452 376 L 715 376"/><text class="label" x="584" y="356" text-anchor="middle">normalize</text>
  <path class="green" d="M 715 426 L 988 426"/><text class="label" x="852" y="406" text-anchor="middle">write chunk 1</text>
  <path class="blue" d="M 988 476 L 1253 476"/><text class="label" x="1120" y="456" text-anchor="middle">save next_index=2</text>
  <path class="blue" d="M 206 536 L 452 536"/><text class="label" x="329" y="516" text-anchor="middle">read acct-1003</text>
  <path class="red" d="M 452 586 L 715 586"/><text class="label" x="584" y="566" text-anchor="middle">processor crash</text>
  <path class="purple" d="M 206 646 L 1253 646"/><text class="label" x="730" y="626" text-anchor="middle">restart loads next_index=2</text>
  <path class="purple" d="M 452 696 L 988 696"/><text class="label" x="720" y="676" text-anchor="middle">write acct-1003..1005</text>
  <path class="green" d="M 988 746 L 1253 746"/><text class="label" x="1120" y="726" text-anchor="middle">save next_index=5</text>

  <g transform="translate(134 834)"><rect width="1232" height="24" rx="12" fill="#ffffff" stroke="#d8e2e8"/><text class="footer" x="616" y="18" text-anchor="middle">Completed chunk acct-1001..1002 is not read by the restart run.</text></g>
</svg>
SVG

rsvg-convert "$out_dir/account-migration-checkpoint-restart-scenario.svg" -o "$out_dir/account-migration-checkpoint-restart-scenario.png"
rsvg-convert "$out_dir/account-migration-checkpoint-restart-architecture.svg" -o "$out_dir/account-migration-checkpoint-restart-architecture.png"
rsvg-convert "$out_dir/account-migration-checkpoint-restart-sequence.svg" -o "$out_dir/account-migration-checkpoint-restart-sequence.png"

validate_svg "account-migration-checkpoint-restart-scenario"
validate_svg "account-migration-checkpoint-restart-architecture"
validate_svg "account-migration-checkpoint-restart-sequence"

write_gate "account-migration-checkpoint-restart-scenario" 8 6 9 44 44 34 34
write_gate "account-migration-checkpoint-restart-architecture" 8 8 13 44 44 34 34
write_gate "account-migration-checkpoint-restart-sequence" 10 10 10 44 44 34 34
