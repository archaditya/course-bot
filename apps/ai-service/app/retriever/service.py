from typing import List, Dict, Any
from app.retriever.qdrant_client import QdrantClient
from app.retriever.bm25 import BM25Retriever
from app.providers import EmbeddingProvider


class RetrieverService:
    def __init__(
        self, 
        qdrant: QdrantClient, 
        bm25: BM25Retriever,
        embedding_provider: EmbeddingProvider,
    ):
        self.qdrant = qdrant
        self.bm25 = bm25
        self.embedding_provider = embedding_provider
    
    async def hybrid_search(
        self,
        query: str,
        course_id: str,
        collection_name: str,
        top_k: int = 10,
        alpha: float = 0.5,  # Weight for vector search (1-alpha for BM25)
    ) -> List[Dict[str, Any]]:
        """Hybrid search combining vector and BM25."""
        # Vector search
        query_embedding = await self.embedding_provider.embed([query])
        vector_results = await self.qdrant.search(
            collection_name=collection_name,
            query_vector=query_embedding[0].embedding,
            course_id=course_id,
            limit=top_k * 2,  # Get more for reranking
        )
        
        # BM25 search
        bm25_results = self.bm25.search(course_id, query, top_k=top_k * 2)
        
        # Combine scores
        combined_scores = self._combine_scores(
            vector_results, 
            bm25_results, 
            alpha
        )
        
        # Sort and return top_k
        sorted_results = sorted(
            combined_scores.items(),
            key=lambda x: x[1],
            reverse=True,
        )[:top_k]
        
        return [
            {
                "chunk_id": chunk_id,
                "document_id": self._get_document_id(chunk_id),
                "score": score,
                "content": self._get_content(chunk_id),
            }
            for chunk_id, score in sorted_results
        ]
    
    def _combine_scores(
        self, 
        vector_results: List[dict], 
        bm25_results: List[tuple],
        alpha: float,
    ) -> Dict[str, float]:
        """Combine vector and BM25 scores using weighted average."""
        scores = {}
        
        # Normalize and add vector scores
        if vector_results:
            max_vec_score = max(r["score"] for r in vector_results)
            for result in vector_results:
                chunk_id = result["chunk_id"]
                normalized = result["score"] / max_vec_score if max_vec_score > 0 else 0
                scores[chunk_id] = scores.get(chunk_id, 0) + alpha * normalized
        
        # Normalize and add BM25 scores
        if bm25_results:
            max_bm25_score = max(score for _, score in bm25_results)
            for idx, score in bm25_results:
                # Get chunk_id from documents (simplified)
                chunk_id = f"chunk_{idx}"  # This should come from actual data
                normalized = score / max_bm25_score if max_bm25_score > 0 else 0
                scores[chunk_id] = scores.get(chunk_id, 0) + (1 - alpha) * normalized
        
        return scores
    
    def _get_document_id(self, chunk_id: str) -> str:
        """Get document_id for a chunk (placeholder)."""
        return "doc_placeholder"
    
    def _get_content(self, chunk_id: str) -> str:
        """Get content for a chunk (placeholder)."""
        return "Content placeholder"