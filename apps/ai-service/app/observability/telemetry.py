import time
import uuid
import logging
from collections import deque
from contextvars import ContextVar
from typing import Dict, Any, List, Optional
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

from config.settings import settings

logger = logging.getLogger("ai-service.telemetry")

# ContextVar to maintain distributed trace context across async tasks
trace_context: ContextVar[str] = ContextVar("trace_id", default="")

def get_current_trace_id() -> str:
    """Retrieve the current distributed trace ID, or generate a fallback."""
    tid = trace_context.get()
    if not tid:
        tid = f"trc-{uuid.uuid4().hex[:12]}"
        trace_context.set(tid)
    return tid


def get_model_rates(model_name: str) -> Dict[str, float]:
    """
    Dynamically resolve cost rates per 1M tokens based on active settings
    and provider types without locking to static model names.
    """
    m = (model_name or "").lower()
    
    # 1. Active embedding model configured in settings
    if settings.openai_embedding_model.lower() in m or "embed" in m:
        return {"input": 0.02, "output": 0.00}
    # 2. Active large LLM configured in settings (e.g. gpt-4o)
    if settings.openai_llm_large_model.lower() in m or "gpt-4o" in m:
        return {"input": 2.50, "output": 10.00}
    # 3. Active mini LLM configured in settings (e.g. gpt-4o-mini)
    if settings.openai_llm_mini_model.lower() in m or "mini" in m:
        return {"input": 0.15, "output": 0.60}
    # 4. Large OSS / Groq 70b
    if "70b" in m:
        return {"input": 0.59, "output": 0.79}
    # 5. Fast OSS / Groq models (gpt-oss-20b, 8b, etc.)
    if settings.groq_llm_fast_model.lower() in m or "gpt-oss-20b" in m or "8b" in m:
        return {"input": 0.05, "output": 0.08}
        
    # Default general fallback rate per 1M tokens ($0.15 in, $0.60 out)
    return {"input": 0.15, "output": 0.60}


class GenAITelemetryTracker:
    """Thread-safe in-memory telemetry aggregator for GenAI metrics and spans."""

    def __init__(self, max_recent_spans: int = 200):
        self.max_recent_spans = max_recent_spans
        self.requests_total: int = 0
        self.errors_total: int = 0
        self.endpoint_stats: Dict[str, Dict[str, Any]] = {}
        
        # Token and cost accounting
        self.total_input_tokens: int = 0
        self.total_output_tokens: int = 0
        self.total_estimated_cost_usd: float = 0.0
        self.tokens_by_model: Dict[str, Dict[str, int]] = {}
        
        # GenAI Span history
        self.recent_spans: deque = deque(maxlen=max_recent_spans)

    def record_request(self, path: str, method: str, status_code: int, duration_ms: float):
        self.requests_total += 1
        if status_code >= 400:
            self.errors_total += 1

        key = f"{method} {path}"
        if key not in self.endpoint_stats:
            self.endpoint_stats[key] = {
                "count": 0,
                "errors": 0,
                "total_duration_ms": 0.0,
                "avg_duration_ms": 0.0,
                "p95_approx_ms": 0.0,
                "recent_latencies": deque(maxlen=100),
            }

        stats = self.endpoint_stats[key]
        stats["count"] += 1
        if status_code >= 400:
            stats["errors"] += 1
        stats["total_duration_ms"] += duration_ms
        stats["avg_duration_ms"] = round(stats["total_duration_ms"] / stats["count"], 2)
        stats["recent_latencies"].append(duration_ms)

        # Approximate p95
        if len(stats["recent_latencies"]) >= 10:
            sorted_latencies = sorted(stats["recent_latencies"])
            idx = int(0.95 * len(sorted_latencies))
            stats["p95_approx_ms"] = round(sorted_latencies[idx], 2)
        else:
            stats["p95_approx_ms"] = round(duration_ms, 2)

    def calculate_cost(self, model: str, input_tokens: int, output_tokens: int) -> float:
        rates = get_model_rates(model)
        cost_in = (input_tokens / 1_000_000.0) * rates["input"]
        cost_out = (output_tokens / 1_000_000.0) * rates["output"]
        return round(cost_in + cost_out, 6)

    def record_span(
        self,
        span_name: str,
        provider: str,
        model: str,
        latency_ms: float,
        input_tokens: int = 0,
        output_tokens: int = 0,
        status: str = "ok",
        trace_id: Optional[str] = None,
        attributes: Optional[Dict[str, Any]] = None,
    ):
        tid = trace_id or get_current_trace_id()
        cost = self.calculate_cost(model, input_tokens, output_tokens)

        # Update totals
        self.total_input_tokens += input_tokens
        self.total_output_tokens += output_tokens
        self.total_estimated_cost_usd += cost

        if model not in self.tokens_by_model:
            self.tokens_by_model[model] = {"input": 0, "output": 0, "cost_usd": 0.0}
        self.tokens_by_model[model]["input"] += input_tokens
        self.tokens_by_model[model]["output"] += output_tokens
        self.tokens_by_model[model]["cost_usd"] += cost

        span_record = {
            "timestamp": time.time(),
            "trace_id": tid,
            "span_name": span_name,
            "provider": provider,
            "model": model,
            "latency_ms": round(latency_ms, 2),
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "cost_usd": cost,
            "status": status,
            "attributes": attributes or {},
        }
        self.recent_spans.append(span_record)

    def get_metrics_summary(self) -> Dict[str, Any]:
        endpoints_summary = {}
        for ep, st in self.endpoint_stats.items():
            endpoints_summary[ep] = {
                "count": st["count"],
                "errors": st["errors"],
                "avg_duration_ms": st["avg_duration_ms"],
                "p95_approx_ms": st["p95_approx_ms"],
            }

        return {
            "service": "archadiLM-ai-service",
            "uptime_status": "healthy",
            "requests_total": self.requests_total,
            "errors_total": self.errors_total,
            "tokens": {
                "total_input_tokens": self.total_input_tokens,
                "total_output_tokens": self.total_output_tokens,
                "total_tokens": self.total_input_tokens + self.total_output_tokens,
                "by_model": self.tokens_by_model,
            },
            "costs": {
                "total_estimated_usd": round(self.total_estimated_cost_usd, 5),
            },
            "endpoints": endpoints_summary,
            "recent_spans_count": len(self.recent_spans),
        }

    def get_recent_traces(self, limit: int = 25) -> List[Dict[str, Any]]:
        spans = list(self.recent_spans)
        return spans[-limit:] if len(spans) > limit else spans


telemetry_tracker = GenAITelemetryTracker()


class TraceContextMiddleware(BaseHTTPMiddleware):
    """FastAPI Middleware to propagate distributed trace IDs and log request spans."""

    async def dispatch(self, request: Request, call_next):
        trace_id = request.headers.get("X-Trace-ID") or request.headers.get("traceparent")
        if not trace_id:
            trace_id = f"trc-{uuid.uuid4().hex[:12]}"

        trace_context.set(trace_id)
        start_time = time.perf_counter()

        try:
            response: Response = await call_next(request)
            duration_ms = (time.perf_counter() - start_time) * 1000.0
            response.headers["X-Trace-ID"] = trace_id

            telemetry_tracker.record_request(
                path=request.url.path,
                method=request.method,
                status_code=response.status_code,
                duration_ms=duration_ms,
            )
            return response
        except Exception as exc:
            duration_ms = (time.perf_counter() - start_time) * 1000.0
            telemetry_tracker.record_request(
                path=request.url.path,
                method=request.method,
                status_code=500,
                duration_ms=duration_ms,
            )
            raise exc


def record_genai_call(
    span_name: str,
    provider: str,
    model: str,
    latency_ms: float,
    input_tokens: int = 0,
    output_tokens: int = 0,
    status: str = "ok",
    attributes: Optional[Dict[str, Any]] = None,
):
    """Utility function to record GenAI telemetry from anywhere in the codebase."""
    telemetry_tracker.record_span(
        span_name=span_name,
        provider=provider,
        model=model,
        latency_ms=latency_ms,
        input_tokens=input_tokens,
        output_tokens=output_tokens,
        status=status,
        attributes=attributes,
    )
