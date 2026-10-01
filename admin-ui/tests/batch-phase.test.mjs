import test from "node:test";
import assert from "node:assert/strict";
import {batchPhaseLabel} from "../src/batch-phase.mjs";

test("batch phases have bilingual labels without claiming acceptance is health", () => {
  for (const phase of ["not-prepared","idle","loading","load-error","uneditable","editing","review","accepted","uncertain","submitting","previewing","blocked","conflict","denied","preview-error","archived","inspecting","reading-conflict","reading-next"]) {
    const en=batchPhaseLabel(phase,"en"),zh=batchPhaseLabel(phase,"zh");
    assert.notEqual(en,phase);assert.match(zh,/[\u4e00-\u9fff]/);
    assert.doesNotMatch(en,/Unknown state/);
  }
  assert.equal(batchPhaseLabel("accepted","zh"),"提交已接受，非健康证明");
  assert.equal(batchPhaseLabel("accepted","en"),"Apply accepted; not health proof");
  assert.equal(batchPhaseLabel("uncertain","zh"),"写入结果未知");
});

test("unknown phases stay explicitly unknown and evidence is not rewritten", () => {
  for(const phase of ["future","constructor","toString","__proto__"]){
    assert.equal(batchPhaseLabel(phase,"en"),`Unknown state (${phase})`);
    assert.equal(batchPhaseLabel(phase,"zh"),`未知状态 (${phase})`);
  }
  for(const phase of [null,undefined,{},""])assert.equal(batchPhaseLabel(phase,"zh"),"未知状态");
  const evidence=Object.freeze({phase:"accepted"});
  batchPhaseLabel(evidence.phase,"zh");assert.deepEqual(evidence,{phase:"accepted"});
});
