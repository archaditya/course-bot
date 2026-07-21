from abc import ABC, abstractmethod
from typing import AsyncGenerator, List
from pydantic import BaseModel


class Prompt(BaseModel):
    text: str
    system_prompt: str | None = None
    temperature: float = 0.7
    max_tokens: int = 1000
    prompt_version: str = "1.0"


class Response(BaseModel):
    text: str
    model: str
    prompt_version: str


class Token(BaseModel):
    text: str
    done: bool = False


class Vector(BaseModel):
    embedding: List[float]
    model: str


class PIIResult(BaseModel):
    has_pii: bool
    detected_types: List[str]
    redacted_text: str | None = None


class InjectionResult(BaseModel):
    is_injection: bool
    confidence: float
    detected_pattern: str | None = None


class RankedChunk(BaseModel):
    chunk_id: str
    document_id: str
    score: float
    content: str
    start_timestamp: int | None = None
    end_timestamp: int | None = None


class LLMProvider(ABC):
    @abstractmethod
    async def generate(self, prompt: Prompt) -> Response:
        pass
    
    @abstractmethod
    async def stream(self, prompt: Prompt) -> AsyncGenerator[Token, None]:
        pass


class EmbeddingProvider(ABC):
    @abstractmethod
    async def embed(self, texts: List[str]) -> List[Vector]:
        pass


class RerankerProvider(ABC):
    @abstractmethod
    async def rerank(
        self, query: str, candidates: List[dict]
    ) -> List[RankedChunk]:
        pass


class GuardrailProvider(ABC):
    @abstractmethod
    async def check_pii(self, text: str) -> PIIResult:
        pass
    
    @abstractmethod
    async def check_injection(self, text: str) -> InjectionResult:
        pass