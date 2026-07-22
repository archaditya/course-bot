from pydantic import BaseModel
from typing import List, Optional


class EmbeddingRequest(BaseModel):
    texts: List[str]


class EmbeddingResponse(BaseModel):
    embeddings: List[List[float]]
    model: str


class TitleGenerationRequest(BaseModel):
    content: str
    prompt_version: Optional[str] = "1.0"


class TitleGenerationResponse(BaseModel):
    title: str


class SummaryGenerationRequest(BaseModel):
    content: str
    prompt_version: Optional[str] = "1.0"


class SummaryGenerationResponse(BaseModel):
    summary: str


class ChunkData(BaseModel):
    chunk_id: str
    document_id: str
    content: str
    start_timestamp: Optional[int] = None
    end_timestamp: Optional[int] = None


class RetrievalRequest(BaseModel):
    query: str
    course_id: str
    collection_name: str
    top_k: Optional[int] = 10


class RetrievalResponse(BaseModel):
    chunks: List[ChunkData]


class RerankRequest(BaseModel):
    query: str
    candidates: List[ChunkData]
    top_k: Optional[int] = 5


class RerankResponse(BaseModel):
    ranked_chunks: List[ChunkData]


class GenerationRequest(BaseModel):
    query: str
    context: str
    prompt_version: Optional[str] = "1.0"


class GenerationResponse(BaseModel):
    content: str  # For non-streaming


class EvaluationRequest(BaseModel):
    query: str
    response: str
    context: str
    prompt_version: Optional[str] = "1.0"


class EvaluationResponse(BaseModel):
    score: float
    passes_threshold: bool


class GuardrailCheckRequest(BaseModel):
    text: str


class GuardrailCheckResponse(BaseModel):
    passed: bool
    reason: str

# ── Query Enhancement ─────────────────────────────────────────────────────

class QueryEnhancementRequest(BaseModel):
    query: str
    prompt_version: Optional[str] = "1.0"


class QueryEnhancementResponse(BaseModel):
    step_back: str
    rewritten: str
    sub_queries: List[str]


# ── HyDE ──────────────────────────────────────────────────────────────────

class HydeDocumentRequest(BaseModel):
    query: str
    prompt_version: Optional[str] = "1.0"


class HydeDocumentResponse(BaseModel):
    document: str

# ── PDF Extraction ────────────────────────────────────────────────────────

class PDFPage(BaseModel):
    page_number: int
    text: str


class PDFExtractionResponse(BaseModel):
    pages: List[PDFPage]

# ── URL Extraction ────────────────────────────────────────────────────────

class URLExtractionRequest(BaseModel):
    url: str


class URLSection(BaseModel):
    text: str
    heading: Optional[str] = None


class URLExtractionResponse(BaseModel):
    title: str
    sections: List[URLSection]
