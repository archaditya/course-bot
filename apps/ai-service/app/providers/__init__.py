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
from config.settings import settings


def get_llm_provider() -> LLMProvider:
    if settings.llm_provider == "openai":
        return OpenAILLM()
    raise ValueError(f"Unknown LLM provider: {settings.llm_provider}")


def get_embedding_provider() -> EmbeddingProvider:
    if settings.embedding_provider == "openai":
        return OpenAIEmbedding()
    raise ValueError(f"Unknown embedding provider: {settings.embedding_provider}")


def get_reranker_provider() -> RerankerProvider:
    if settings.reranker_provider == "openai":
        return OpenAIReranker()
    raise ValueError(f"Unknown reranker provider: {settings.reranker_provider}")


def get_guardrail_provider() -> GuardrailProvider:
    if settings.guardrail_provider == "openai":
        return OpenAIGuardrail()
    raise ValueError(f"Unknown guardrail provider: {settings.guardrail_provider}")