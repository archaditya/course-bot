import os
from pathlib import Path
from dotenv import load_dotenv
from pydantic_settings import BaseSettings
from typing import Literal

# Load root .env file (B:\Personal-Projects\GenAI\Course-Bot\.env)
root_dir = Path(__file__).resolve().parent.parent.parent
root_env = root_dir / ".env"
if root_env.exists():
    load_dotenv(dotenv_path=root_env, override=True)
load_dotenv()


class Settings(BaseSettings):
    # API
    api_host: str = os.getenv("AI_SERVICE_HOST", "0.0.0.0")
    api_port: int = int(os.getenv("AI_SERVICE_PORT", "8000"))
    
    # OpenAI
    openai_api_key: str = os.getenv("OPENAI_API_KEY", "")
    openai_embedding_model: str = "text-embedding-3-small"
    openai_llm_large_model: str = "gpt-4o"
    openai_llm_mini_model: str = "gpt-4o-mini"

    # Groq API (Active models as of Aug 2026 — all Llama 3.x models decommissioned)
    groq_api_key: str = os.getenv("GROQ_API_KEY", "")
    groq_llm_large_model: str = os.getenv("GROQ_LLM_LARGE_MODEL", "openai/gpt-oss-20b")
    groq_llm_fast_model: str = os.getenv("GROQ_LLM_FAST_MODEL", "openai/gpt-oss-20b")
    
    # Qdrant
    qdrant_url: str = os.getenv("QDRANT_URL", "http://localhost:6333")
    qdrant_api_key: str | None = os.getenv("QDRANT_API_KEY", None)
    
    # Feature Flags
    guardrails_enabled: bool = True
    evaluator_enabled: bool = True
    max_retries: int = 3
    evaluator_threshold: float = 7.0
    
    # Provider Selection
    llm_provider: Literal["openai"] = "openai"
    embedding_provider: Literal["openai"] = "openai"
    reranker_provider: Literal["openai"] = "openai"
    guardrail_provider: Literal["openai"] = "openai"

    # YouTube API
    youtube_api_key: str = os.getenv("YOUTUBE_API_KEY", "")

    # Web Extraction & Search APIs
    firecrawl_api_key: str = os.getenv("FIRECRAWL_API_KEY", "")
    tavily_api_key: str = os.getenv("TAVILY_API_KEY", "")



settings = Settings()

if not settings.openai_api_key:
    print("⚠️ WARNING: OPENAI_API_KEY is not set in root .env!")
