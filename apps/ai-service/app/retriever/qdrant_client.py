from qdrant_client import AsyncQdrantClient
from qdrant_client.models import Distance, VectorParams, PointStruct, Filter, FieldCondition, MatchValue
from typing import List, Optional
from config.settings import settings


class QdrantClient:
    def __init__(self):
        self.client = AsyncQdrantClient(
            url=settings.qdrant_url,
            api_key=settings.qdrant_api_key,
        )
    
    async def create_collection(self, collection_name: str, vector_size: int = 1536):
        """Create a collection if it doesn't exist."""
        collections = await self.client.get_collections()
        existing = [c.name for c in collections.collections]
        
        if collection_name not in existing:
            await self.client.create_collection(
                collection_name=collection_name,
                vectors_config=VectorParams(size=vector_size, distance=Distance.COSINE),
            )
    
    async def upsert_points(
        self, collection_name: str, points: List[PointStruct]
    ):
        """Upsert points to collection."""
        await self.client.upsert(
            collection_name=collection_name,
            points=points,
        )
    
    async def search(
        self,
        collection_name: str,
        query_vector: List[float],
        course_id: str,
        limit: int = 10,
        score_threshold: float = 0.7,
    ) -> List[dict]:
        """Search with course_id filter."""
        results = await self.client.search(
            collection_name=collection_name,
            query_vector=query_vector,
            query_filter=Filter(
                must=[
                    FieldCondition(
                        key="course_id",
                        match=MatchValue(value=course_id),
                    )
                ]
            ),
            limit=limit,
            score_threshold=score_threshold,
        )
        
        return [
            {
                "chunk_id": hit.payload["chunk_id"],
                "document_id": hit.payload["document_id"],
                "score": hit.score,
                "start_timestamp": hit.payload.get("start_timestamp"),
                "end_timestamp": hit.payload.get("end_timestamp"),
            }
            for hit in results
        ]