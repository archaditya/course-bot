from app.providers.base import (
    LLMProvider,
    EmbeddingProvider,
    RerankerProvider,
    GuardrailProvider,
    Prompt,
    Response,
    Vector,
    Token,
    PIIResult,
    InjectionResult,
    RankedChunk,
)
from app.providers.openai.client import (
    OpenAILLM,
    OpenAIEmbedding,
    OpenAIReranker,
    OpenAIGuardrail,
)
from app.providers.groq.client import GroqLLM, GroqGuardrail
from config.settings import settings


def get_llm_provider() -> LLMProvider:
    """Fast provider (Groq) for guardrails, query enhancement, HyDE, title gen."""
    if settings.groq_api_key:
        return GroqLLM()
    return OpenAILLM()


def get_smart_llm_provider() -> LLMProvider:
    """Quality provider (OpenAI) for user-facing generation, evaluation, and source intelligence."""
    return OpenAILLM()


def get_embedding_provider() -> EmbeddingProvider:
    return OpenAIEmbedding()  # Always OpenAI for Qdrant 1536-dim vectors


def get_reranker_provider() -> RerankerProvider:
    return OpenAIReranker()


def get_guardrail_provider() -> GuardrailProvider:
    """Fast guardrail checks (Groq) — PII, injection detection."""
    if settings.groq_api_key:
        return GroqGuardrail()
    return OpenAIGuardrail()
