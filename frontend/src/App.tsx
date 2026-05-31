import { FormEvent, useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";

import { analyzeReviewStream } from "./api/reviewStream";
import { AITracePanel } from "./components/AITracePanel";
import { EvidenceDrawer } from "./components/EvidenceDrawer";
import { PrInputPanel } from "./components/PrInputPanel";
import { ReviewBrief } from "./components/ReviewBrief";
import { RiskRadar } from "./components/RiskRadar";
import { StatusBanner } from "./components/StatusBanner";
import { StepTimeline } from "./components/StepTimeline";
import { SuggestedComments } from "./components/SuggestedComments";
import { initialReviewState, reviewReducer } from "./state/reviewReducer";
import type { ReviewEvent } from "./types/review";
import { getVisibleRisks, type RiskSeverityFilter } from "./utils/riskFilters";
import "./styles.css";

// App 连接输入表单、流式分析状态和报告展示区域。
export default function App() {
  const [state, dispatch] = useReducer(reviewReducer, initialReviewState);
  const [prURL, setPrURL] = useState("");
  const [token, setToken] = useState("");
  const [isRunning, setIsRunning] = useState(false);
  const [activeRiskId, setActiveRiskId] = useState<string | null>(null);
  const [riskFilter, setRiskFilter] = useState<RiskSeverityFilter>("all");
  const abortRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);

  // runAnalysis 取消旧请求并启动新的 SSE 分析会话。
  async function runAnalysis(request: { pr_url: string; github_token?: string; demo: boolean }) {
    abortRef.current?.abort();
    dispatch({ type: "reset" });
    setIsRunning(true);

    const requestId = requestIdRef.current + 1;
    requestIdRef.current = requestId;

    const controller = new AbortController();
    abortRef.current = controller;

    // 只处理当前请求的事件，避免慢响应覆盖新会话状态。
    const dispatchIfCurrent = (event: ReviewEvent) => {
      if (requestIdRef.current === requestId) {
        dispatch({ type: "stream_event", event });
      }
    };

    try {
      await analyzeReviewStream(request, dispatchIfCurrent, controller.signal);
    } catch (error) {
      if (requestIdRef.current !== requestId) {
        return;
      }

      dispatch({
        type: "stream_event",
        event: {
          type: "error",
          data: {
            code: "stream_request_failed",
            message: error instanceof Error ? error.message : "Review stream failed",
            recoverable: true
          }
        }
      });
    } finally {
      if (requestIdRef.current === requestId) {
        setIsRunning(false);
        abortRef.current = null;
      }
    }
  }

  // submit 将表单输入转换成真实 PR 分析请求。
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await runAnalysis({ pr_url: prURL, github_token: token || undefined, demo: false });
  }

  // runDemo 启动后端提供的确定性演示流。
  async function runDemo() {
    await runAnalysis({ pr_url: "", demo: true });
  }

  const closeEvidence = useCallback(() => {
    const riskId = activeRiskId;
    setActiveRiskId(null);

    if (!riskId) {
      return;
    }

    window.requestAnimationFrame(() => {
      document.querySelector<HTMLElement>(`[data-risk-id="${CSS.escape(riskId)}"]`)?.focus();
    });
  }, [activeRiskId]);

  const report = state.result;
  // 规则风险到达后立即展示；最终报告生成后优先展示合并后的风险。
  const risks = report?.risks ?? state.ruleRisks;
  const visibleRisks = useMemo(() => getVisibleRisks(risks, riskFilter), [riskFilter, risks]);
  const activeRisk = useMemo(
    () => visibleRisks.find((risk) => risk.id === activeRiskId) ?? null,
    [activeRiskId, visibleRisks]
  );

  // 当前过滤条件隐藏已选风险时，清空详情抽屉的选中态。
  useEffect(() => {
    if (activeRiskId && !visibleRisks.some((risk) => risk.id === activeRiskId)) {
      setActiveRiskId(null);
    }
  }, [activeRiskId, visibleRisks]);

  return (
    <main className="app-shell">
      <PrInputPanel
        prURL={prURL}
        token={token}
        isRunning={isRunning}
        onPrURLChange={setPrURL}
        onTokenChange={setToken}
        onAnalyze={submit}
        onRunDemo={runDemo}
      />

      <section className="workspace">
        <StepTimeline steps={state.steps} />

        <section className="report">
          <StatusBanner error={state.error} degraded={state.degraded} result={report} />

          {report ? (
            <ReviewBrief report={report} />
          ) : (
            <div className={isRunning ? "empty-report empty-report--running" : "empty-report"}>
              <div>
                <h2>{isRunning ? "Analysis in progress" : "No report yet"}</h2>
                <p>
                  {isRunning
                    ? "Findings can appear before the final normalized report."
                    : "Enter a PR URL or run the demo stream."}
                </p>
                {isRunning ? (
                  <div className="empty-report__skeleton" aria-hidden="true">
                    <span />
                    <span />
                    <span />
                  </div>
                ) : null}
              </div>
            </div>
          )}

          <div className="review-evidence-grid">
            <RiskRadar
              risks={risks}
              activeRiskId={activeRiskId}
              filter={riskFilter}
              onFilterChange={setRiskFilter}
              onSelectRisk={(risk) => setActiveRiskId(risk.id)}
            />
            <EvidenceDrawer risk={activeRisk} onClose={closeEvidence} />
          </div>

          {report ? <SuggestedComments report={report} /> : null}

          <AITracePanel aiText={state.aiText} steps={state.steps} />
        </section>
      </section>
    </main>
  );
}
