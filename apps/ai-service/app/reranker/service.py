from typing import List, Dict
from app.providers import RerankerProvider, RankedChunk


class RerankerService:
    def __init__(self, provider: RerankerProvider):
        self.provider = provider
    
    async def rerank(
        self, 
        query: str, 
        candidates: List[Dict],
        top_k: int = 5,
    ) -> List[RankedChunk]:
        """Rerank candidates by relevance to query."""
        if not candidates:
            return []
        
        ranked = await self.provider.rerank(query, candidates)
        return ranked[:top_k]