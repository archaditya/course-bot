from fastapi import FastAPI, HTTPException, UploadFile, File
from fastapi.responses import StreamingResponse
from typing import AsyncGenerator
import json

from api.schemas import (
    EmbeddingRequest, EmbeddingResponse,
    TitleGenerationRequest, TitleGenerationResponse,
    SummaryGenerationRequest, SummaryGenerationResponse,
    RetrievalRequest, RetrievalResponse,
    RerankRequest, RerankResponse,
    GenerationRequest, GenerationResponse,
    EvaluationRequest, EvaluationResponse,
    GuardrailCheckRequest, GuardrailCheckResponse,
    QueryEnhancementRequest, QueryEnhancementResponse,
    HydeDocumentRequest, HydeDocumentResponse,
)

from app.providers import get_embedding_provider, get_llm_provider, get_reranker_provider, get_guardrail_provider
from app.embedding.service import EmbeddingService
from app.title_generator.service import TitleGeneratorService
from app.summary_generator.service import SummaryGeneratorService
from app.retriever.service import RetrieverService
from app.retriever.qdrant_client import QdrantClient
from app.retriever.bm25 import BM25Retriever
from app.reranker.service import RerankerService
from app.generator.service import GeneratorService
from app.evaluator.service import EvaluatorService
from app.guardrails.service import GuardrailsService
from app.query_enhancer.service import QueryEnhancerService
from app.hyde.service import HydeService
from app.pdf_extractor.service import PDFExtractorService
from app.url_extractor.service import URLExtractorService
from api.schemas import URLExtractionRequest, URLExtractionResponse, URLSection as URLSectionSchema

app = FastAPI(title="archadiLM AI Service", version="0.2.0")

# Initialize services
embedding_provider = get_embedding_provider()
llm_provider = get_llm_provider()
reranker_provider = get_reranker_provider()
guardrail_provider = get_guardrail_provider()

embedding_service = EmbeddingService(embedding_provider)
title_generator = TitleGeneratorService(llm_provider)
summary_generator = SummaryGeneratorService(llm_provider)
reranker = RerankerService(reranker_provider)
generator = GeneratorService(llm_provider)
evaluator = EvaluatorService(llm_provider)
guardrails = GuardrailsService(guardrail_provider)
query_enhancer = QueryEnhancerService(llm_provider)
hyde_service = HydeService(llm_provider)
pdf_extractor = PDFExtractorService()
url_extractor = URLExtractorService()

qdrant = QdrantClient()
bm25 = BM25Retriever()
retriever = RetrieverService(qdrant, bm25, embedding_provider)


@app.get("/healthz")
async def health_check():
    return {"status": "ok", "service": "ai-service", "version": "0.2.0"}


@app.post("/embeddings", response_model=EmbeddingResponse)
async def create_embeddings(request: EmbeddingRequest):
    """Generate embeddings for texts."""
    try:
        vectors = await embedding_service.embed_texts(request.texts)
        return EmbeddingResponse(
            embeddings=[v.embedding for v in vectors],
            model=vectors[0].model if vectors else "",
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/generate-title", response_model=TitleGenerationResponse)
async def generate_title(request: TitleGenerationRequest):
    """Generate a title for content."""
    try:
        title = await title_generator.generate_title(
            request.content, 
            request.prompt_version
        )
        return TitleGenerationResponse(title=title)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/generate-summary", response_model=SummaryGenerationResponse)
async def generate_summary(request: SummaryGenerationRequest):
    """Generate a summary for content."""
    try:
        summary = await summary_generator.generate_summary(
            request.content,
            request.prompt_version
        )
        return SummaryGenerationResponse(summary=summary)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/retrieve", response_model=RetrievalResponse)
async def retrieve(request: RetrievalRequest):
    """Hybrid retrieval (vector + BM25)."""
    try:
        chunks = await retriever.hybrid_search(
            query=request.query,
            course_id=request.course_id,
            collection_name=request.collection_name,
            top_k=request.top_k,
        )
        return RetrievalResponse(
            chunks=[
                {
                    "chunk_id": c["chunk_id"],
                    "document_id": c["document_id"],
                    "content": c["content"],
                    "start_timestamp": c.get("start_timestamp"),
                    "end_timestamp": c.get("end_timestamp"),
                }
                for c in chunks
            ]
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/rerank", response_model=RerankResponse)
async def rerank(request: RerankRequest):
    """Rerank candidates by relevance."""
    try:
        candidates = [c.dict() for c in request.candidates]
        ranked = await reranker.rerank(
            query=request.query,
            candidates=candidates,
            top_k=request.top_k,
        )
        return RerankResponse(
            ranked_chunks=[
                {
                    "chunk_id": r.chunk_id,
                    "document_id": r.document_id,
                    "content": r.content,
                    "start_timestamp": r.start_timestamp,
                    "end_timestamp": r.end_timestamp,
                }
                for r in ranked
            ]
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/generate")
async def generate(request: GenerationRequest):
    """Generate response with streaming."""
    async def stream_response() -> AsyncGenerator[str, None]:
        try:
            async for token in generator.generate(
                query=request.query,
                context=request.context,
                prompt_version=request.prompt_version,
            ):
                if token.done:
                    yield "[DONE]"
                else:
                    yield token.text
        except Exception as e:
            yield f"[ERROR: {str(e)}]"
    
    return StreamingResponse(
        stream_response(),
        media_type="text/plain",
    )


@app.post("/evaluate", response_model=EvaluationResponse)
async def evaluate(request: EvaluationRequest):
    """Evaluate response quality."""
    try:
        score = await evaluator.evaluate(
            query=request.query,
            response=request.response,
            context=request.context,
            prompt_version=request.prompt_version,
        )
        return EvaluationResponse(
            score=score,
            passes_threshold=evaluator.passes_threshold(score),
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/guardrails/check", response_model=GuardrailCheckResponse)
async def check_guardrails(request: GuardrailCheckRequest):
    """Check text against guardrails."""
    try:
        passed, reason = await guardrails.check_query(request.text)
        return GuardrailCheckResponse(passed=passed, reason=reason)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# ── NEW: Advanced RAG Pipeline Endpoints ──────────────────────────────────


@app.post("/enhance-query", response_model=QueryEnhancementResponse)
async def enhance_query(request: QueryEnhancementRequest):
    """Query understanding: step-back, rewrite, sub-query decomposition."""
    try:
        result = await query_enhancer.enhance(
            query=request.query,
            prompt_version=request.prompt_version,
        )
        return QueryEnhancementResponse(
            step_back=result.step_back,
            rewritten=result.rewritten,
            sub_queries=result.sub_queries,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/hyde-document", response_model=HydeDocumentResponse)
async def hyde_document(request: HydeDocumentRequest):
    """Generate a hypothetical document for HyDE embedding."""
    try:
        document = await hyde_service.generate(
            query=request.query,
            prompt_version=request.prompt_version,
        )
        return HydeDocumentResponse(document=document)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/extract-pdf")
async def extract_pdf(file: UploadFile = File(...)):
    """Extract text from a PDF file page-by-page."""
    try:
        content = await file.read()
        pages = await pdf_extractor.extract(content)
        return {"pages": [p.dict() for p in pages]}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/extract-url", response_model=URLExtractionResponse)
async def extract_url(request: URLExtractionRequest):
    """Extract readable text from a web URL."""
    try:
        result = await url_extractor.extract(request.url)
        return URLExtractionResponse(
            title=result.title,
            sections=[
                URLSectionSchema(text=s.text, heading=s.heading)
                for s in result.sections
            ],
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))