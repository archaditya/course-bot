import logging
from typing import Any, Dict, List

import httpx
from pydantic import BaseModel

from config.settings import settings

logger = logging.getLogger("ai-service.web-search")


class WebSearchResultItem(BaseModel):
    title: str
    url: str
    content: str


class WebSearchResult(BaseModel):
    answer: str
    results: List[WebSearchResultItem]


class WebSearchService:
    """Performs real-time web search via Tavily Search API."""

    async def search(self, query: str, max_results: int = 5) -> WebSearchResult:
        tavily_key = getattr(settings, "tavily_api_key", None) or getattr(settings, "TAVILY_API_KEY", None)
        if not tavily_key:
            logger.warning("Tavily API key not configured")
            return WebSearchResult(answer="", results=[])

        try:
            async with httpx.AsyncClient(timeout=15.0) as client:
                resp = await client.post(
                    "https://api.tavily.com/search",
                    headers={"Content-Type": "application/json"},
                    json={
                        "api_key": tavily_key,
                        "query": query,
                        "include_answer": True,
                        "search_depth": "basic",
                        "max_results": max_results,
                    },
                )
                if resp.status_code != 200:
                    logger.error(f"Tavily status {resp.status_code}: {resp.text[:200]}")
                    return WebSearchResult(answer="", results=[])

                data = resp.json()
                answer = data.get("answer", "")
                results_raw = data.get("results", [])

                items: List[WebSearchResultItem] = []
                for item in results_raw:
                    items.append(
                        WebSearchResultItem(
                            title=item.get("title", ""),
                            url=item.get("url", ""),
                            content=item.get("content", ""),
                        )
                    )

                return WebSearchResult(answer=answer, results=items)
        except Exception as e:
            logger.error(f"Tavily search error: {e}")
            return WebSearchResult(answer="", results=[])
