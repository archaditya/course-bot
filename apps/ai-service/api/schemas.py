from pydantic import BaseModel, Field
from typing import List, Optional

# ── Max String Length Constants ──────────────────────────────────────────
MAX_SHORT_TEXT = 4000      # Queries, chat inputs, guardrail checks
MAX_MEDIUM_TEXT = 50000    # Individual document chunks or section text
MAX_LONG_TEXT = 200000     # Full document contents for intel / summarization


class EmbeddingRequest(BaseModel):
    texts: List[str] = Field(..., min_items=1, max_items=128, description="List of texts to embed")


class EmbeddingResponse(BaseModel):
    embeddings: List[List[float]]
    model: str


class TitleGenerationRequest(BaseModel):
    content: str = Field(..., min_length=1, max_length=MAX_MEDIUM_TEXT)
    prompt_version: Optional[str] = "1.0"


class TitleGenerationResponse(BaseModel):
    title: str


class SummaryGenerationRequest(BaseModel):
    content: str = Field(..., min_length=1, max_length=MAX_LONG_TEXT)
    prompt_version: Optional[str] = "1.0"


class SummaryGenerationResponse(BaseModel):
    summary: str


class SourceIntelRequest(BaseModel):
    content: str = Field(..., min_length=1, max_length=MAX_LONG_TEXT)
    filename: str = Field(..., min_length=1, max_length=255)
    prompt_version: Optional[str] = "1.0"


class SourceIntelResponse(BaseModel):
    summary: str
    questions: List[str]
    overview: str


class ChunkData(BaseModel):
    chunk_id: str
    document_id: Optional[str] = None
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
    query: str = Field(..., min_length=1, max_length=MAX_SHORT_TEXT)
    context: str = Field(..., max_length=MAX_MEDIUM_TEXT)
    prompt_version: Optional[str] = "1.0"


class GenerationResponse(BaseModel):
    content: str  # For non-streaming


class EvaluationRequest(BaseModel):
    query: str = Field(..., min_length=1, max_length=MAX_SHORT_TEXT)
    response: str = Field(..., min_length=1, max_length=MAX_MEDIUM_TEXT)
    context: str = Field(..., max_length=MAX_MEDIUM_TEXT)
    prompt_version: Optional[str] = "1.0"


class EvaluationResponse(BaseModel):
    score: float
    passes_threshold: bool


class GuardrailCheckRequest(BaseModel):
    text: str = Field(..., min_length=1, max_length=MAX_SHORT_TEXT, description="User text to check against guardrails")


class GuardrailCheckResponse(BaseModel):
    passed: bool
    reason: str

# ── Query Enhancement ─────────────────────────────────────────────────────

class QueryEnhancementRequest(BaseModel):
    query: str = Field(..., min_length=1, max_length=MAX_SHORT_TEXT)
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

class IntentClassifyRequest(BaseModel):
    query: str
    has_documents: bool = False

class IntentClassifyResponse(BaseModel):
    intent: str
    confidence: float
    reasoning: str
    used_llm: bool

class MemorySearchRequest(BaseModel):
    workspace_id: str
    query: str

class MemorySearchResponse(BaseModel):
    memories: List[str]

class MemoryAddRequest(BaseModel):
    workspace_id: str
    user_text: str
    assistant_text: str

class MemoryAddResponse(BaseModel):
    added_facts: List[str]

# ── Learning Tools & Web Search ──────────────────────────────────────────

class LearningToolRequest(BaseModel):
    tool_type: str
    content: str
    title: Optional[str] = ""

class LearningToolResponse(BaseModel):
    result: dict | list

class WebSearchRequest(BaseModel):
    query: str

class WebSearchItemSchema(BaseModel):
    title: str
    url: str
    content: str

class WebSearchResponse(BaseModel):
    answer: str
    results: List[WebSearchItemSchema]
