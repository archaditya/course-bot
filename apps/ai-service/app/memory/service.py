import os
import asyncio
import logging
from typing import List
from config.settings import settings

# Explicit safeguard before mem0 import
os.environ["MEM0_TELEMETRY"] = "true" if settings.mem0_telemetry else "false"

# Suppress harmless optional dependency warnings from mem0 (we use OpenAI LLM + Embeddings)
logging.getLogger("mem0.vector_stores.qdrant").setLevel(logging.ERROR)
logging.getLogger("mem0.utils.spacy_models").setLevel(logging.ERROR)

logger = logging.getLogger(__name__)

try:
    from mem0 import Memory
    MEM0_AVAILABLE = True
except ImportError:
    MEM0_AVAILABLE = False
    logger.warning("mem0ai library not installed. Please run pip install mem0ai")


class WorkspaceMemoryService:
    def __init__(self):
        self.memory = None
        if MEM0_AVAILABLE:
            try:
                config = {
                    "vector_store": {
                        "provider": "qdrant",
                        "config": {
                            "url": settings.qdrant_url,
                            "api_key": settings.qdrant_api_key,
                        }
                    },
                    "llm": {
                        "provider": "openai",
                        "config": {
                            "model": settings.openai_llm_mini_model,
                            "api_key": settings.openai_api_key,
                        }
                    },
                    "embedder": {
                        "provider": "openai",
                        "config": {
                            "model": settings.openai_embedding_model,
                            "api_key": settings.openai_api_key,
                        }
                    }
                }
                self.memory = Memory.from_config(config)
            except Exception as e:
                logger.error(f"Failed to initialize Mem0 client: {e}")
                # Fallback to default memory instance
                try:
                    self.memory = Memory()
                except Exception:
                    self.memory = None

    def _sync_search(self, workspace_id: str, query: str) -> List[str]:
        try:
            results = self.memory.search(query=query, filters={"user_id": workspace_id})
            if isinstance(results, dict) and "results" in results:
                return [m["memory"] for m in results["results"] if "memory" in m]
            elif isinstance(results, list):
                return [m["memory"] if isinstance(m, dict) and "memory" in m else str(m) for m in results]
            return []
        except Exception as e:
            logger.error(f"Mem0 search error: {e}")
            return []

    def _sync_add(self, workspace_id: str, user_text: str, assistant_text: str) -> List[str]:
        try:
            messages = [
                {"role": "user", "content": user_text},
                {"role": "assistant", "content": assistant_text},
            ]
            self.memory.add(messages, user_id=workspace_id)
            return ["Memory added"]
        except Exception as e:
            logger.error(f"Mem0 add error: {e}")
            return []

    async def search_memory(self, workspace_id: str, query: str) -> List[str]:
        if not self.memory:
            return []
        return await asyncio.to_thread(self._sync_search, workspace_id, query)

    async def add_memory(self, workspace_id: str, user_text: str, assistant_text: str) -> List[str]:
        if not self.memory:
            return []
        return await asyncio.to_thread(self._sync_add, workspace_id, user_text, assistant_text)
