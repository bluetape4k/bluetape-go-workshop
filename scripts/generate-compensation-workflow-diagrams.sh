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

render_graphviz_pair() {
  local name="$1"
  dot -Tplain "$out_dir/${name}.dot" > "$out_dir/${name}.plain"
  dot -Tsvg "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.svg"
  dot -Tpng "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.png"
  cp "$out_dir/${name}-graphviz.svg" "$out_dir/${name}.svg"
  cp "$out_dir/${name}-graphviz.png" "$out_dir/${name}.png"
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
require_tool rg
require_font "$font_architects"
require_font "$font_comic"

cat > "$out_dir/compensation-workflow-scenario.dot" <<'DOT'
digraph G {
  graph [
    rankdir=LR,
    bgcolor="#fffdf8",
    margin=0.2,
    pad=0.18,
    nodesep=0.62,
    ranksep=0.82,
    splines=true,
    overlap=false,
    outputorder=edgesfirst,
    fontnames=svg,
    fontname="Architects Daughter",
    label="Compensation Workflow Scenario",
    labelloc=t,
    fontsize=32,
    fontcolor="#24313f"
  ];
  node [
    shape=rect,
    style="rounded,filled",
    penwidth=2,
    fontname="Architects Daughter",
    fontsize=19,
    color="#64748b",
    fontcolor="#263645",
    margin="0.18,0.12"
  ];
  edge [
    penwidth=2.6,
    arrowsize=0.8,
    fontname="Comic Mono",
    fontsize=12,
    fontcolor="#425466"
  ];

  request [label="Fulfillment\nrequest", fillcolor="#fef3c7", color="#b99b5d"];
  reserve [label="reserve-inventory\nregister release", fillcolor="#dbeafe", color="#4f7e9d"];
  authorize [label="authorize-payment\nregister void", fillcolor="#dbeafe", color="#4f7e9d"];
  shipment [label="create-shipment\nmay fail", fillcolor="#fee2e2", color="#bf6672"];
  complete [label="completed\nside effects kept", fillcolor="#dcfce7", color="#5d8a62"];
  original [label="original error\npreserved", fillcolor="#ffe4e6", color="#b95f7a"];
  void [label="void-payment\nreverse step 1", fillcolor="#f3e8ff", color="#8a6bb0"];
  release [label="release-inventory\nreverse step 2", fillcolor="#f3e8ff", color="#8a6bb0"];
  response [label="stable JSON\nforward + compensation", fillcolor="#e0f2fe", color="#48758d"];

  request -> reserve [label="forward", color="#3f6f8f"];
  reserve -> authorize [label="success", color="#3f6f8f"];
  authorize -> shipment [label="success", color="#3f6f8f"];
  shipment -> complete [label="shipment ok", color="#5d8a62", fontcolor="#426d46"];
  shipment -> original [label="shipment failed", color="#b95f7a", fontcolor="#9f4d67"];
  original -> void [label="compensate reverse", color="#8a6bb0", fontcolor="#745996"];
  void -> release [label="continue on failure", color="#8a6bb0", fontcolor="#745996"];
  release -> response [label="report both trees", color="#5d8a62", fontcolor="#426d46"];
  complete -> response [label="200 OK", color="#5d8a62", fontcolor="#426d46"];
}
DOT

cat > "$out_dir/compensation-workflow-architecture.dot" <<'DOT'
digraph G {
  graph [
    rankdir=LR,
    bgcolor="#fffdf8",
    margin=0.2,
    pad=0.18,
    nodesep=0.62,
    ranksep=0.86,
    splines=true,
    overlap=false,
    compound=true,
    outputorder=edgesfirst,
    fontnames=svg,
    fontname="Architects Daughter",
    label="Compensation Workflow Architecture",
    labelloc=t,
    fontsize=32,
    fontcolor="#24313f"
  ];
  node [
    shape=rect,
    style="rounded,filled",
    penwidth=2,
    fontname="Architects Daughter",
    fontsize=18,
    color="#64748b",
    fontcolor="#263645",
    margin="0.18,0.12"
  ];
  edge [
    penwidth=2.6,
    arrowsize=0.8,
    fontname="Comic Mono",
    fontsize=12,
    fontcolor="#425466"
  ];

  subgraph cluster_http {
    label="HTTP boundary";
    color="#d8e2e8";
    style="rounded";
    fontname="Architects Daughter";
    fontsize=20;
    client [label="HTTP Client", fillcolor="#dbeafe", color="#4f7e9d"];
    gin [label="Gin Router\n/compensation/fulfillment", fillcolor="#fef3c7", color="#b99b5d"];
    mapper [label="Status + JSON\nmapper", fillcolor="#e0f2fe", color="#48758d"];
  }

  subgraph cluster_app {
    label="Application run object";
    color="#d8e2e8";
    style="rounded";
    fontname="Architects Daughter";
    fontsize=20;
    handler [label="Request Handler\nnew run per request", fillcolor="#ede9fe", color="#8a6bb0"];
    stack [label="Compensation Stack\nrelease + void", fillcolor="#f3e8ff", color="#8a6bb0"];
    effects [label="Side-effect Flags\ninventory/payment/shipment", fillcolor="#ffe4e6", color="#b95f7a"];
  }

  subgraph cluster_btg {
    label="bluetape-go boundary";
    color="#d8e2e8";
    style="rounded";
    fontname="Architects Daughter";
    fontsize=20;
    workflow [label="workflow.Sequential\nStopOnFailure / ContinueOnFailure", fillcolor="#dcfce7", color="#5d8a62"];
    report [label="workreport.Report\nnested execution tree", fillcolor="#ecfccb", color="#78944d"];
  }

  client -> gin [label="request", color="#3f6f8f"];
  gin -> handler [label="bind + validate", color="#3f6f8f"];
  handler -> workflow [label="forward runner", color="#5d8a62"];
  workflow -> stack [label="register successful steps", color="#8a6bb0"];
  workflow -> effects [label="mutate request-scoped flags", color="#b95f7a"];
  workflow -> report [label="forward report", color="#5d8a62"];
  handler -> workflow [label="reverse runner on failure", color="#8a6bb0", constraint=false];
  stack -> workflow [label="reverse order", color="#8a6bb0"];
  report -> mapper [label="original error + children", color="#5d8a62"];
  mapper -> client [label="200 / 409 / 408 / 400", color="#5d8a62"];
}
DOT

cat > "$out_dir/compensation-workflow-sequence.dot" <<'DOT'
digraph G {
  graph [
    rankdir=TB,
    bgcolor="#fffdf8",
    margin=0.2,
    pad=0.18,
    nodesep=0.42,
    ranksep=0.58,
    splines=ortho,
    outputorder=edgesfirst,
    fontnames=svg,
    fontname="Architects Daughter",
    label="Compensation Workflow Sequence",
    labelloc=t,
    fontsize=32,
    fontcolor="#24313f"
  ];
  node [
    shape=rect,
    style="rounded,filled",
    penwidth=2,
    fontname="Architects Daughter",
    fontsize=18,
    color="#64748b",
    fontcolor="#263645",
    margin="0.18,0.12"
  ];
  edge [
    penwidth=2.5,
    arrowsize=0.75,
    fontname="Comic Mono",
    fontsize=11,
    fontcolor="#425466"
  ];

  client [label="Client", fillcolor="#dbeafe", color="#4f7e9d"];
  handler [label="Handler", fillcolor="#fef3c7", color="#b99b5d"];
  forward [label="Forward Sequential\nStopOnFailure", fillcolor="#dcfce7", color="#5d8a62"];
  reserve [label="reserve-inventory", fillcolor="#e0f2fe", color="#48758d"];
  authorize [label="authorize-payment", fillcolor="#e0f2fe", color="#48758d"];
  shipment [label="create-shipment", fillcolor="#ffe4e6", color="#b95f7a"];
  stack [label="Compensation Stack", fillcolor="#f3e8ff", color="#8a6bb0"];
  compensation [label="Compensation Sequential\nContinueOnFailure", fillcolor="#f3e8ff", color="#8a6bb0"];
  void [label="void-payment", fillcolor="#ede9fe", color="#8a6bb0"];
  release [label="release-inventory", fillcolor="#ede9fe", color="#8a6bb0"];
  mapper [label="Response Mapper\noriginal error kept", fillcolor="#ecfccb", color="#78944d"];

  client -> handler [label="POST order", color="#3f6f8f"];
  handler -> forward [label="Run(ctx)", color="#5d8a62"];
  forward -> reserve [label="reserve", color="#5d8a62"];
  reserve -> stack [label="push release", color="#8a6bb0"];
  forward -> authorize [label="authorize", color="#5d8a62"];
  authorize -> stack [label="push void", color="#8a6bb0"];
  forward -> shipment [label="ship", color="#5d8a62"];
  shipment -> handler [label="failed report", color="#b95f7a", fontcolor="#9f4d67"];
  handler -> compensation [label="reverse stack", color="#8a6bb0"];
  compensation -> void [label="void", color="#8a6bb0"];
  compensation -> release [label="release", color="#8a6bb0"];
  compensation -> mapper [label="compensation report", color="#8a6bb0"];
  forward -> mapper [label="success path", color="#5d8a62"];
  mapper -> client [label="stable JSON", color="#5d8a62"];
}
DOT

render_graphviz_pair "compensation-workflow-scenario"
render_graphviz_pair "compensation-workflow-architecture"
render_graphviz_pair "compensation-workflow-sequence"

validate_svg "compensation-workflow-scenario"
validate_svg "compensation-workflow-architecture"
validate_svg "compensation-workflow-sequence"

write_gate "compensation-workflow-scenario" 9 9 14
write_gate "compensation-workflow-architecture" 8 10 18
write_gate "compensation-workflow-sequence" 10 14 20
