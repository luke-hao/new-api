"""Validate a frozen legacy sample, including async workers and buffered quota."""
import argparse,json
from pathlib import Path
PREFIX="github.com/QuantumNous/new-api/"
SLEEP={"common.(*InMemoryRateLimiter).clearExpiredItems","service.TaskPollingLoop","controller.UpdateMidjourneyTaskBulk","model.InitBatchUpdater.func1","controller.AutomaticallyTestChannels.func1","model.UpdateQuotaData","model.SyncOptions","common.StartSystemMonitor.func1","pkg/perf_metrics.flushLoop","model.SyncChannelCache"}
WAIT={"service.StartSubscriptionQuotaResetTask.func1.1","controller.StartChannelUpstreamModelUpdateTask.func1.1","service.StartCodexCredentialAutoRefreshTask.func1.1","controller.StartChannelGroupStabilityTask.func1.1","service.startCleanupTask.func1"}
def validate(r):
    reasons=[]
    for k in ("batch_entries","batch_locks"):
        if any(r[k]):reasons.append(k)
    for k in ("dashboard_entries","dashboard_lock","pool_task_head_present","pool_task_count"):
        if r[k]:reasons.append(k)
    prebody=0
    for g in r["all_goroutines"]:
        if g["status"]!=4:reasons.append("active_goroutine");continue
        if not g.get("stack_complete",False):reasons.append("incomplete_stack");continue
        app=g["application"];top=g["top"]
        if "unknown" in top:reasons.append("unknown_stack");continue
        if not app:
            if g["http_handler"]:reasons.append("http_handler")
            if g.get("pool_worker"):reasons.append("unclassified_pool_worker")
            continue
        first=app[0].removeprefix(PREFIX)
        # Distribute reads the body before entering controller.Relay and before precharge.
        if g["http_handler"] and first=="common.CreateBodyStorageFromReader" and not any("controller.Relay" in x for x in app):prebody+=1;continue
        if first in SLEEP and "time.Sleep" in top:continue
        if first in WAIT and ("runtime.chanrecv" in top or "runtime.selectgo" in top):continue
        reasons.append(first)
    return {"passed":not reasons,"blocking_reasons":sorted(set(reasons)),"unsubmitted_body_readers":prebody,"requires_frozen_process":True}
if __name__=="__main__":
    p=argparse.ArgumentParser();p.add_argument("report");a=p.parse_args();print(json.dumps(validate(json.loads(Path(a.report).read_text())),indent=2))
