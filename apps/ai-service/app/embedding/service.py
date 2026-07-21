from typing import List
from app.providers import EmbeddingProvider, Vector
from app.providers.base import Prompt


class EmbeddingService:
    def __init__(self, provider: EmbeddingProvider):
        self.provider = provider
    
    async def embed_texts(self, texts: List[str]) -> List[Vector]:
        """Generate embeddings for a list of texts."""
        if not texts:
            return []
        
        return await self.provider.embed(texts)
    
    async def embed_single(self, text: str) -> Vector:
        """Generate embedding for a single text."""
        result = await self.provider.embed([text])
        return result[0]