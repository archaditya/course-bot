from rank_bm25 import BM25Okapi
from typing import List, Tuple
import re


class BM25Retriever:
    def __init__(self):
        self.indexes = {}  # course_id -> BM25Okapi
        self.documents = {}  # course_id -> List[dict]
    
    def build_index(self, course_id: str, documents: List[dict]):
        """Build BM25 index for a course."""
        tokenized_docs = [
            self._tokenize(doc["content"]) 
            for doc in documents
        ]
        self.indexes[course_id] = BM25Okapi(tokenized_docs)
        self.documents[course_id] = documents
    
    def search(
        self, course_id: str, query: str, top_k: int = 10
    ) -> List[Tuple[int, float]]:
        """Search using BM25, returns (doc_index, score)."""
        if course_id not in self.indexes:
            return []
        
        tokenized_query = self._tokenize(query)
        scores = self.indexes[course_id].get_scores(tokenized_query)
        
        top_indices = sorted(
            range(len(scores)), key=lambda i: scores[i], reverse=True
        )[:top_k]
        
        return [(idx, scores[idx]) for idx in top_indices]
    
    def _tokenize(self, text: str) -> List[str]:
        """Simple tokenization."""
        text = text.lower()
        text = re.sub(r'[^\w\s]', ' ', text)
        return text.split()