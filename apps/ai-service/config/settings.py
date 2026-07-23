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
    api_host: str = os.getenv("AI_SERVICE_HOST", "127.0.0.1")
    api_port: int = int(os.getenv("AI_SERVICE_PORT", "8000"))
    
    # OpenAI
    openai_api_key: str = os.getenv("OPENAI_API_KEY", "")
    openai_embedding_model: str = "text-embedding-3-small"
    openai_llm_large_model: str = "gpt-4o"
    openai_llm_mini_model: str = "gpt-4o-mini"
    
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


settings = Settings()

if not settings.openai_api_key:
    print("⚠️ WARNING: OPENAI_API_KEY is not set in root .env!")
